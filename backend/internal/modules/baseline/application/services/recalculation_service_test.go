package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

var fixedNow = at(10000)

func newRecalc(history *fakeHistory, baselines *fakeBaselines, events *fakeEvents) *RecalculationService {
	return NewRecalculationService(history, baselines, events, testConfig(), testLogger(), func() time.Time { return fixedNow })
}

func stableHistory() *fakeHistory { return &fakeHistory{samples: normalSamples(at(0), at(24*20))} }

func TestRecalculate_CreatesAVersionFromTheHistoryWindow(t *testing.T) {
	history, baselines, events := stableHistory(), newFakeBaselines(), &fakeEvents{}
	asOf := at(24 * 20)

	got, err := newRecalc(history, baselines, events).
		Recalculate(context.Background(), RecalculateCommand{PointID: 11556, AsOf: asOf, Trigger: entities.TriggerManual})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(history.calls) != 1 || !history.calls[0].from.Equal(at(0)) || !history.calls[0].to.Equal(asOf) {
		t.Fatalf("histórico consultado = %+v, se esperaba [%v, %v)", history.calls, at(0), asOf)
	}
	if len(events.calls) != 1 || !events.calls[0].from.Equal(at(-2)) || !events.calls[0].to.Equal(asOf.Add(2*time.Hour)) {
		t.Fatalf("eventos consultados = %+v, se esperaba el rango con ±2 h", events.calls)
	}

	if !got.Created || got.SkippedReason != "" || len(baselines.saved) != 1 {
		t.Fatalf("resultado = %+v, guardadas = %d", got, len(baselines.saved))
	}
	v := got.Version
	if v.ID != 1 || v.MeasuringPointID != 11556 || !v.ValidFrom.Equal(asOf) || !v.HistoryFrom.Equal(at(0)) || !v.HistoryTo.Equal(asOf) {
		t.Fatalf("versión mal armada: %+v", v)
	}
	if v.Trigger != entities.TriggerManual || v.AlgorithmVersion != features.AlgorithmVersion || !v.CreatedAt.Equal(fixedNow) {
		t.Fatalf("trigger=%s algoritmo=%s creada=%v", v.Trigger, v.AlgorithmVersion, v.CreatedAt)
	}
	if v.Confidence != entities.ConfidenceNormal || v.Signal.MedianB2 != 50 || got.Confidence != v.Confidence || len(got.Passes) == 0 {
		t.Fatalf("estimación no propagada: %+v", got)
	}
}

func TestRecalculate_DryRunNeverPersists(t *testing.T) {
	baselines := newFakeBaselines()

	got, err := newRecalc(stableHistory(), baselines, &fakeEvents{}).
		Recalculate(context.Background(), RecalculateCommand{PointID: 1, AsOf: at(24 * 20), DryRun: true})

	if err != nil || got.Created || got.SkippedReason != SkippedDryRun || len(baselines.saved) != 0 {
		t.Fatalf("dry run: resultado=%+v err=%v guardadas=%d", got, err, len(baselines.saved))
	}
	if got.Version.Signal.MedianB2 != 50 {
		t.Fatal("el dry run igual debe devolver la versión calculada")
	}
}

func TestRecalculate_InsufficientHistoryIsSkippedWithoutError(t *testing.T) {
	baselines := newFakeBaselines()

	got, err := newRecalc(&fakeHistory{}, baselines, &fakeEvents{}).
		Recalculate(context.Background(), RecalculateCommand{PointID: 1, AsOf: at(24 * 20)})

	if err != nil || got.Created || got.SkippedReason != SkippedInsufficientHistory || len(baselines.saved) != 0 {
		t.Fatalf("resultado=%+v err=%v", got, err)
	}
}

func TestRecalculate_ConcurrentUpdateIsANoOp(t *testing.T) {
	baselines := newFakeBaselines()
	baselines.saveErr = fmt.Errorf("envuelto: %w", domain.ErrBaselineConcurrentUpdate)

	got, err := newRecalc(stableHistory(), baselines, &fakeEvents{}).
		Recalculate(context.Background(), RecalculateCommand{PointID: 1, AsOf: at(24 * 20)})

	if err != nil || got.Created || got.SkippedReason != SkippedConcurrentUpdate {
		t.Fatalf("resultado=%+v err=%v", got, err)
	}
}

func TestRecalculate_Errors(t *testing.T) {
	tests := []struct {
		name      string
		pointID   int64
		history   *fakeHistory
		events    *fakeEvents
		baselines func() *fakeBaselines
		want      error
	}{
		{name: "punto inválido", pointID: 0, history: stableHistory(), events: &fakeEvents{}, want: domain.ErrInvalidMeasuringPointID},
		{name: "falla el histórico", pointID: 1, history: &fakeHistory{err: errDB}, events: &fakeEvents{}, want: domain.ErrConsultingHistory},
		{name: "fallan los eventos conocidos", pointID: 1, history: stableHistory(), events: &fakeEvents{err: errDB}, want: domain.ErrConsultingKnownEvents},
		{
			name: "falla al guardar", pointID: 1, history: stableHistory(), events: &fakeEvents{},
			baselines: func() *fakeBaselines { b := newFakeBaselines(); b.saveErr = errDB; return b },
			want:      domain.ErrPersistingBaseline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baselines := newFakeBaselines()
			if tt.baselines != nil {
				baselines = tt.baselines()
			}

			got, err := newRecalc(tt.history, baselines, tt.events).
				Recalculate(context.Background(), RecalculateCommand{PointID: tt.pointID, AsOf: at(24 * 20)})

			if !errors.Is(err, tt.want) || got.Created {
				t.Fatalf("error=%v creado=%v, se esperaba %v", err, got.Created, tt.want)
			}
		})
	}
}

func TestRecalculateDue_DecidesPerPointAndKeepsGoingOnErrors(t *testing.T) {
	now := at(24 * 20)
	history := stableHistory()
	history.points = []int64{1, 2, 3}
	baselines := newFakeBaselines()
	baselines.latestErr[1] = errDB
	baselines.latest[2] = &entities.BaselineVersion{CreatedAt: now.Add(-time.Hour), Confidence: entities.ConfidenceNormal}
	// el punto 3 no tiene versión: bootstrap

	got, err := newRecalc(history, baselines, &fakeEvents{}).RecalculateDue(context.Background(), now)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("se esperaba un resumen por punto: %+v", got)
	}

	if !errors.Is(got[0].Err, domain.ErrConsultingBaseline) || got[0].Due {
		t.Fatalf("punto 1: %+v", got[0])
	}
	if got[1].Due || got[1].Result != nil || got[1].Err != nil {
		t.Fatalf("punto 2 no vencido: %+v", got[1])
	}
	if !got[2].Due || got[2].Trigger != entities.TriggerBootstrap || got[2].Result == nil || !got[2].Result.Created {
		t.Fatalf("punto 3 debía crearse con bootstrap: %+v", got[2])
	}
	if len(baselines.saved) != 1 || baselines.saved[0].MeasuringPointID != 3 || baselines.saved[0].Trigger != entities.TriggerBootstrap {
		t.Fatalf("versiones guardadas = %+v", baselines.saved)
	}
}

func TestRecalculateDue_ListErrorFails(t *testing.T) {
	_, err := newRecalc(&fakeHistory{listErr: errDB}, newFakeBaselines(), &fakeEvents{}).
		RecalculateDue(context.Background(), at(0))

	if !errors.Is(err, domain.ErrConsultingHistory) {
		t.Fatalf("error = %v, se esperaba ErrConsultingHistory", err)
	}
}

func TestRecalculateDue_StopsWhenTheContextIsCanceled(t *testing.T) {
	history := stableHistory()
	history.points = []int64{1, 2}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := newRecalc(history, newFakeBaselines(), &fakeEvents{}).RecalculateDue(ctx, at(24*20))

	if err != nil || len(got) != 0 || len(history.calls) != 0 {
		t.Fatalf("con el contexto cancelado no se procesa ningún punto: %+v (err %v)", got, err)
	}
}

func TestNewRecalculationService_DefaultsTheClock(t *testing.T) {
	s := NewRecalculationService(&fakeHistory{}, newFakeBaselines(), &fakeEvents{}, testConfig(), testLogger(), nil)

	if s.now == nil || time.Since(s.now()) > time.Minute {
		t.Fatal("sin reloj se debe usar time.Now")
	}
}
