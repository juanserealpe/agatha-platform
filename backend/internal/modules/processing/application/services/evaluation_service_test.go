package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

var ctx = context.Background()

// withHistory deja al punto con 5 días de datos hasta now y la última hora
// evaluada en last (filas previas sembradas directamente en el fake).
func withHistory(f *fixture, pointID int64, last time.Time) {
	f.reader.samples[pointID] = data(f.now.Add(-5*24*time.Hour), f.now)
	for _, w := range []int{24, 6} {
		f.evals.rows[evalKey{pointID, w, last}] = entities.FeatureEvaluation{MeasuringPointID: pointID, WindowHours: w, EvaluatedAt: last}
	}
}

func event(pointID int64, readyUntil time.Time) EvaluatePendingCommand {
	return EvaluatePendingCommand{PointID: pointID, ReadyUntil: readyUntil, Trigger: entities.TriggerEvent, MaxSlots: 24}
}

func TestEvaluatePendingEvaluatesTwoRowsPerHour(t *testing.T) {
	f := newFixture(h0.Add(10 * time.Minute))
	withHistory(f, pointA, h0.Add(-3*time.Hour))

	res, err := f.service.EvaluatePending(ctx, event(pointA, h0))
	if err != nil {
		t.Fatal(err)
	}

	if res.Planned != 3 || res.Inserted != 6 || !res.FirstSlot.Equal(h0.Add(-2*time.Hour)) || !res.LastSlot.Equal(h0) {
		t.Fatalf("resultado = %+v", res)
	}
	if f.reader.samplesCall != 1 {
		t.Fatalf("lecturas de samples = %d, se esperaba una sola", f.reader.samplesCall)
	}

	e24, ok := f.evals.get(pointA, 24, h0)
	if !ok || e24.Status != entities.StatusCalculated || e24.Trigger != entities.TriggerEvent || e24.I == nil {
		t.Fatalf("fila 24 h = %+v", e24)
	}
	if e6, ok := f.evals.get(pointA, 6, h0); !ok || !e6.CFrom24h {
		t.Fatalf("fila 6 h = %+v", e6)
	}
}

func TestEvaluatePendingIsIdempotent(t *testing.T) {
	f := newFixture(h0.Add(10 * time.Minute))
	withHistory(f, pointA, h0.Add(-3*time.Hour))

	if _, err := f.service.EvaluatePending(ctx, event(pointA, h0)); err != nil {
		t.Fatal(err)
	}
	samplesCalls := f.reader.samplesCall

	res, err := f.service.EvaluatePending(ctx, event(pointA, h0))
	if err != nil || res.Planned != 0 {
		t.Fatalf("segunda llamada: planned = %d, err = %v", res.Planned, err)
	}
	if f.reader.samplesCall != samplesCalls || f.evals.saveCalls != 1 {
		t.Fatal("la segunda llamada no debe leer samples ni guardar")
	}
}

func TestEvaluatePendingNeverEvaluatesTheFuture(t *testing.T) {
	f := newFixture(h0.Add(10 * time.Minute))
	withHistory(f, pointA, h0.Add(-2*time.Hour))

	res, err := f.service.EvaluatePending(ctx, event(pointA, h0.Add(5*time.Hour)))
	if err != nil || !res.LastSlot.Equal(h0) {
		t.Fatalf("última hora = %v (err %v), se esperaba floorHour(now) = %v", res.LastSlot, err, h0)
	}
}

func TestEvaluatePendingDerivesReadyUntilFromData(t *testing.T) {
	f := newFixture(h0.Add(50 * time.Minute))
	withHistory(f, pointA, h0.Add(-2*time.Hour))
	// Datos hasta h0 − 15 min: la hora h0 todavía no tiene watermark ≥ t.
	f.reader.samples[pointA] = data(h0.Add(-5*24*time.Hour), h0)

	res, err := f.service.EvaluatePending(ctx, EvaluatePendingCommand{PointID: pointA, Trigger: entities.TriggerEvent, MaxSlots: 24})
	if err != nil || res.Planned != 1 || !res.LastSlot.Equal(h0.Add(-time.Hour)) {
		t.Fatalf("resultado = %+v, err = %v", res, err)
	}
}

func TestEvaluatePendingWithoutHistoryUsesInitialLookback(t *testing.T) {
	f := newFixture(h0.Add(10 * time.Minute))
	f.reader.samples[pointA] = data(h0.Add(-5*24*time.Hour), f.now)

	res, err := f.service.EvaluatePending(ctx, EvaluatePendingCommand{PointID: pointA, ReadyUntil: h0, Trigger: entities.TriggerReconciler, MaxSlots: 0})
	if err != nil || res.Planned != 72 || !res.FirstSlot.Equal(h0.Add(-71*time.Hour)) {
		t.Fatalf("resultado = %+v, err = %v", res, err)
	}
}

func TestEvaluatePendingFailuresPersistNothing(t *testing.T) {
	tests := []struct {
		name    string
		break_  func(f *fixture)
		wantErr error
	}{
		{"falla el provider a mitad del lote", func(f *fixture) { at := h0.Add(-time.Hour); f.baseline.failAt = &at }, domain.ErrConsultingBaseline},
		{"falla la lectura de samples", func(f *fixture) { f.reader.samplesErr = errDB }, domain.ErrConsultingMeasurements},
		{"falla la última hora evaluada", func(f *fixture) { f.evals.lastErr = errDB }, domain.ErrConsultingEvaluations},
		{"falla el guardado", func(f *fixture) { f.evals.saveErr = errDB }, domain.ErrPersistingEvaluations},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(h0.Add(10 * time.Minute))
			withHistory(f, pointA, h0.Add(-3*time.Hour))
			tt.break_(f)

			_, err := f.service.EvaluatePending(ctx, event(pointA, h0))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, se esperaba %v", err, tt.wantErr)
			}
			if got := f.evals.count(pointA); got != 2 {
				t.Fatalf("filas = %d: no debía persistirse ninguna nueva", got)
			}
		})
	}
}

func TestEvaluatePendingInvalidPointDoesNotTouchStorage(t *testing.T) {
	f := newFixture(h0)

	_, err := f.service.EvaluatePending(ctx, event(0, h0))
	if !errors.Is(err, domain.ErrInvalidMeasuringPointID) {
		t.Fatalf("error = %v", err)
	}
	if f.reader.samplesCall+f.reader.latestCall+f.evals.saveCalls != 0 {
		t.Fatal("no debía consultar la BD")
	}
}

func TestEvaluatePendingConflictIsNotAnError(t *testing.T) {
	f := newFixture(h0.Add(10 * time.Minute))
	withHistory(f, pointA, h0.Add(-3*time.Hour))
	f.evals.preexisting[evalKey{pointA, 24, h0}] = true // otro proceso ya la insertó

	res, err := f.service.EvaluatePending(ctx, event(pointA, h0))
	if err != nil || res.Inserted != 5 || res.Planned != 3 {
		t.Fatalf("resultado = %+v, err = %v", res, err)
	}
}

func TestEvaluatePendingConcurrentEventAndReconciler(t *testing.T) {
	f := newFixture(h0.Add(40 * time.Minute))
	f.reader.points = []int64{pointA}
	withHistory(f, pointA, h0.Add(-10*time.Hour))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = f.service.EvaluatePending(ctx, event(pointA, h0))
		}()
		go func() {
			defer wg.Done()
			_, _ = f.service.Reconcile(ctx, f.now)
		}()
	}
	wg.Wait()

	if f.evals.conflicts != 0 {
		t.Fatalf("conflictos = %d: el lock por punto debía serializar evento y reconciliador", f.evals.conflicts)
	}
	if missing := f.evals.missing(pointA, h0.Add(-9*time.Hour), h0); len(missing) != 0 {
		t.Fatalf("horas sin fila: %v", missing)
	}
}

func TestReconcileRecoversLostEvent(t *testing.T) {
	f := newFixture(h0.Add(35 * time.Minute))
	f.reader.points = []int64{pointA}
	withHistory(f, pointA, h0.Add(-3*time.Hour))

	summary, err := f.service.Reconcile(ctx, f.now)
	if err != nil || len(summary) != 1 || summary[0].Err != nil {
		t.Fatalf("summary = %+v, err = %v", summary, err)
	}
	if missing := f.evals.missing(pointA, h0.Add(-2*time.Hour), h0); len(missing) != 0 {
		t.Fatalf("horas sin fila: %v", missing)
	}
	if e, _ := f.evals.get(pointA, 24, h0); e.Trigger != entities.TriggerReconciler {
		t.Fatalf("trigger = %s", e.Trigger)
	}
}

func TestReconcileDeviceWithoutDataProducesLowCoverageAfterGrace(t *testing.T) {
	last := h0.Add(-2 * time.Hour)

	t.Run("antes del grace no evalúa la hora", func(t *testing.T) {
		f := newFixture(h0.Add(20 * time.Minute))
		f.reader.points = []int64{pointA}
		withHistory(f, pointA, last)
		f.reader.samples[pointA] = nil

		if _, err := f.service.Reconcile(ctx, f.now); err != nil {
			t.Fatal(err)
		}
		if _, ok := f.evals.get(pointA, 24, h0); ok {
			t.Fatal("h0 no debía evaluarse antes de h0 + 30 min")
		}
		if e, ok := f.evals.get(pointA, 24, h0.Add(-time.Hour)); !ok || e.NoCalcReason != entities.ReasonLowCoverage {
			t.Fatalf("h0 − 1 h = %+v", e)
		}
	})

	t.Run("tras el grace la hora queda LOW_COVERAGE", func(t *testing.T) {
		f := newFixture(h0.Add(35 * time.Minute))
		f.reader.points = []int64{pointA}
		withHistory(f, pointA, last)
		f.reader.samples[pointA] = nil

		if _, err := f.service.Reconcile(ctx, f.now); err != nil {
			t.Fatal(err)
		}
		for _, w := range []int{24, 6} {
			e, ok := f.evals.get(pointA, w, h0)
			if !ok || e.Status != entities.StatusNoCalculable || e.NoCalcReason != entities.ReasonLowCoverage || e.NValid != 0 {
				t.Fatalf("%d h = %+v", w, e)
			}
		}
	})
}

func TestReconcileRecoversLongOutageInTwoTicks(t *testing.T) {
	f := newFixture(h0.Add(35 * time.Minute))
	f.reader.points = []int64{pointA}
	withHistory(f, pointA, h0.Add(-100*time.Hour))
	f.reader.samples[pointA] = data(h0.Add(-130*time.Hour), f.now)

	if _, err := f.service.Reconcile(ctx, f.now); err != nil {
		t.Fatal(err)
	}
	if got := f.evals.count(pointA); got != 2+72*2 {
		t.Fatalf("tras el primer tick filas = %d, se esperaban %d (tope 72)", got, 2+72*2)
	}

	if _, err := f.service.Reconcile(ctx, f.now); err != nil {
		t.Fatal(err)
	}
	if missing := f.evals.missing(pointA, h0.Add(-99*time.Hour), h0); len(missing) != 0 {
		t.Fatalf("horas sin fila tras dos ticks: %v", missing)
	}
}

func TestReconcileErrorInOnePointDoesNotStopOthers(t *testing.T) {
	f := newFixture(h0.Add(35 * time.Minute))
	f.reader.points = []int64{pointA, pointB}
	withHistory(f, pointA, h0.Add(-2*time.Hour))
	withHistory(f, pointB, h0.Add(-2*time.Hour))
	f.reader.latestErr[pointA] = errDB

	summary, err := f.service.Reconcile(ctx, f.now)
	if err != nil || len(summary) != 2 {
		t.Fatalf("summary = %+v, err = %v", summary, err)
	}
	if !errors.Is(summary[0].Err, domain.ErrConsultingMeasurements) || summary[1].Err != nil {
		t.Fatalf("errores = %v / %v", summary[0].Err, summary[1].Err)
	}
	if _, ok := f.evals.get(pointB, 24, h0); !ok {
		t.Fatal("el punto B debía evaluarse")
	}
}

func TestReconcileListFailure(t *testing.T) {
	f := newFixture(h0)
	f.reader.listErr = errDB

	if _, err := f.service.Reconcile(ctx, f.now); !errors.Is(err, domain.ErrConsultingMeasurements) {
		t.Fatalf("error = %v", err)
	}
}

func TestReconcileReadyUntil(t *testing.T) {
	grace := 30 * time.Minute
	data := h0.Add(-3 * time.Hour).Add(45 * time.Minute)

	tests := []struct {
		name   string
		now    time.Time
		latest *time.Time
		want   time.Time
	}{
		{"datos al día", h0.Add(5 * time.Minute), ptr(h0), h0},
		{"sin datos antes del grace", h0.Add(20 * time.Minute), nil, h0.Add(-time.Hour)},
		{"sin datos tras el grace", h0.Add(30 * time.Minute), nil, h0},
		{"datos atrasados tras el grace", h0.Add(40 * time.Minute), &data, h0},
		{"datos futuros no superan floorHour(now)", h0.Add(5 * time.Minute), ptr(h0.Add(3 * time.Hour)), h0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reconcileReadyUntil(tt.now, tt.latest, grace); !got.Equal(tt.want) {
				t.Fatalf("readyUntil = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
