package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
)

// hour0 es la primera hora de las secuencias (posterior a day1, la v1 del seed).
var hour0 = time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

func hour(n int) time.Time { return hour0.Add(time.Duration(n) * time.Hour) }

type fakePublisher struct {
	exceeded []contract.LeakThresholdExceeded
	cleared  []contract.LeakThresholdCleared
	err      error
}

func (p *fakePublisher) PublishLeakThresholdExceeded(_ context.Context, e contract.LeakThresholdExceeded) error {
	p.exceeded = append(p.exceeded, e)
	return p.err
}

func (p *fakePublisher) PublishLeakThresholdCleared(_ context.Context, e contract.LeakThresholdCleared) error {
	p.cleared = append(p.cleared, e)
	return p.err
}

type fixture struct {
	thresholds  *memory.ThresholdRepository
	evaluations *memory.RuleEvaluationRepository
	publisher   *fakePublisher
	svc         *services.EvaluationService
}

func newFixture(versions ...entities.ThresholdVersion) *fixture {
	if len(versions) == 0 {
		versions = []entities.ThresholdVersion{seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams())}
	}
	f := &fixture{
		thresholds:  memory.NewThresholdRepository(versions...),
		evaluations: memory.NewRuleEvaluationRepository(),
		publisher:   &fakePublisher{},
	}
	f.svc = services.NewEvaluationService(
		f.thresholds,
		f.evaluations,
		services.NewThresholdService(f.thresholds, fixedClock(), testLogger()),
		f.publisher,
		fixedClock(),
		testLogger(),
	)
	return f
}

func calculated(pointID int64, at time.Time, i float64) services.IndicatorEvaluatedNotice {
	return services.IndicatorEvaluatedNotice{
		MeasuringPointID: pointID,
		WindowHours:      24,
		EvaluatedAt:      at,
		Status:           "CALCULATED",
		I:                &i,
		Trigger:          "RECONCILER",
		OccurredAt:       at.Add(35 * time.Minute),
	}
}

func (f *fixture) feed(t *testing.T, values ...float64) []services.EvaluationResult {
	t.Helper()
	out := make([]services.EvaluationResult, 0, len(values))
	for n, i := range values {
		r, err := f.svc.EvaluateIndicator(context.Background(), calculated(pointA, hour(n), i))
		if err != nil {
			t.Fatalf("hora %d (I=%.2f): %v", n, i, err)
		}
		out = append(out, r)
	}
	return out
}

func TestEvaluateIndicator_IgnoresWindow6hAndNoCalculable(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	w6 := calculated(pointA, hour(0), 0.9)
	w6.WindowHours = 6

	noCalc := calculated(pointA, hour(0), 0)
	noCalc.Status, noCalc.NoCalcReason, noCalc.I = "NO_CALCULABLE", "LOW_COVERAGE", nil

	for _, n := range []services.IndicatorEvaluatedNotice{w6, noCalc} {
		r, err := f.svc.EvaluateIndicator(ctx, n)
		if err != nil || r.Outcome != services.OutcomeIgnored {
			t.Fatalf("outcome = %s, err = %v; se esperaba IGNORED", r.Outcome, err)
		}
	}
	if rows := f.evaluations.Rows(pointA); len(rows) != 0 {
		t.Fatalf("filas = %d, no debía crearse ninguna", len(rows))
	}
}

func TestEvaluateIndicator_PublishesExceededAndClearedOnlyWhenDue(t *testing.T) {
	f := newFixture()

	// NORMAL, entrada pendiente, abre, sube, histéresis, salida 1/2, cierra, NORMAL.
	results := f.feed(t, 0.12, 0.19, 0.20, 0.30, 0.17, 0.15, 0.14, 0.10)

	rows := f.evaluations.Rows(pointA)
	if len(rows) != 8 {
		t.Fatalf("filas = %d, se esperaban 8", len(rows))
	}

	wantEmitted := []entities.Emission{"NONE", "NONE", "EXCEEDED", "EXCEEDED", "NONE", "NONE", "CLEARED", "NONE"}
	for n, row := range rows {
		if row.Emitted != wantEmitted[n] {
			t.Errorf("hora %d: emitted = %s, se esperaba %s", n, row.Emitted, wantEmitted[n])
		}
		if results[n].Published != (wantEmitted[n] != entities.EmittedNone) {
			t.Errorf("hora %d: published = %v", n, results[n].Published)
		}
	}
	if !results[2].EpisodeOpened {
		t.Error("la hora 2 abre el episodio")
	}

	if len(f.publisher.exceeded) != 2 || len(f.publisher.cleared) != 1 {
		t.Fatalf("eventos: exceeded=%d cleared=%d; se esperaban 2 y 1", len(f.publisher.exceeded), len(f.publisher.cleared))
	}

	opened := f.publisher.exceeded[0]
	want := contract.LeakThresholdExceeded{
		RuleEvaluationID:   rows[2].ID,
		MeasuringPointID:   pointA,
		EventType:          "FUGA",
		EvaluatedAt:        hour(2),
		Level:              "AMARILLO",
		PreviousLevel:      "NORMAL",
		Rule:               "THRESHOLD",
		IndicatorValue:     0.20,
		Thresholds:         contract.Thresholds{Amarillo: 0.18, Naranja: 0.25, Rojo: 0.35},
		ThresholdVersionID: rows[2].ThresholdVersionID,
		HoursAbove:         2,
		HoursInLevel:       1,
		OccurredAt:         now,
		SchemaVersion:      contract.SchemaVersion,
	}
	if opened != want {
		t.Fatalf("exceeded = %+v\nse esperaba  %+v", opened, want)
	}
	if up := f.publisher.exceeded[1]; up.Level != "NARANJA" || up.PreviousLevel != "AMARILLO" || up.HoursAbove != 3 {
		t.Fatalf("segundo exceeded = %+v", up)
	}

	cleared := f.publisher.cleared[0]
	if cleared.RuleEvaluationID != rows[6].ID || cleared.LastLevel != "NARANJA" || cleared.HoursAbove != 6 ||
		!cleared.EvaluatedAt.Equal(hour(6)) || cleared.IndicatorValue != 0.14 || cleared.EventType != "FUGA" ||
		cleared.SchemaVersion != contract.SchemaVersion || cleared.ThresholdVersionID == 0 {
		t.Fatalf("cleared = %+v", cleared)
	}
}

func TestEvaluateIndicator_DuplicateDoesNotCreateRowNorRepublish(t *testing.T) {
	f := newFixture()
	f.feed(t, 0.20)

	notice := calculated(pointA, hour(1), 0.21)
	for range 2 {
		if _, err := f.svc.EvaluateIndicator(context.Background(), notice); err != nil {
			t.Fatal(err)
		}
	}

	r, err := f.svc.EvaluateIndicator(context.Background(), notice)
	if err != nil || r.Outcome != services.OutcomeDuplicate {
		t.Fatalf("outcome = %s, err = %v; se esperaba DUPLICATE", r.Outcome, err)
	}
	if rows := f.evaluations.Rows(pointA); len(rows) != 2 {
		t.Fatalf("filas = %d, se esperaban 2", len(rows))
	}
	if len(f.publisher.exceeded) != 1 {
		t.Fatalf("exceeded = %d: el duplicado no se republica", len(f.publisher.exceeded))
	}
}

func TestEvaluateIndicator_LateArrivalIsSavedButNotPublished(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	// Llega primero la hora 5 y después las horas 3 y 4 (desordenadas).
	if _, err := f.svc.EvaluateIndicator(ctx, calculated(pointA, hour(5), 0.10)); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{3, 4} {
		r, err := f.svc.EvaluateIndicator(ctx, calculated(pointA, hour(n), 0.40))
		if err != nil {
			t.Fatal(err)
		}
		if r.Outcome != services.OutcomeSaved || !r.Late || r.Published {
			t.Fatalf("hora %d: %+v; se esperaba guardada, tardía y sin publicar", n, r)
		}
	}

	rows := f.evaluations.Rows(pointA)
	if len(rows) != 3 || rows[1].Emitted != entities.EmittedExceeded {
		t.Fatalf("filas = %+v: la hora tardía se interpreta igual (la 4 abre el episodio)", rows)
	}
	if len(f.publisher.exceeded)+len(f.publisher.cleared) != 0 {
		t.Fatal("una llegada tardía no publica")
	}
}

func TestEvaluateIndicator_NoCalculableGapPausesTheRun(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	steps := []services.IndicatorEvaluatedNotice{
		calculated(pointA, hour(0), 0.19),
		{MeasuringPointID: pointA, WindowHours: 24, EvaluatedAt: hour(1), Status: "NO_CALCULABLE", NoCalcReason: "LOW_COVERAGE"},
		calculated(pointA, hour(2), 0.20),
	}
	for _, n := range steps {
		if _, err := f.svc.EvaluateIndicator(ctx, n); err != nil {
			t.Fatal(err)
		}
	}

	rows := f.evaluations.Rows(pointA)
	if len(rows) != 2 || rows[1].Level != entities.LevelAmarillo || rows[1].HoursAbove != 2 {
		t.Fatalf("filas = %+v: el hueco no genera fila y la racha sigue", rows)
	}
}

func TestEvaluateIndicator_UsesTheVersionEffectiveAtEvaluatedAt(t *testing.T) {
	change := hour(2)
	strict := entities.DefaultThresholdParams()
	strict.Amarillo, strict.Naranja, strict.Rojo = 0.30, 0.40, 0.50
	f := newFixture(
		seedVersion(pointA, 1, day1, &change, entities.DefaultThresholdParams()),
		seedVersion(pointA, 2, change, nil, strict),
	)

	f.feed(t, 0.20, 0.20, 0.20)

	rows := f.evaluations.Rows(pointA)
	v1, v2 := f.thresholds.All()[0].ID, f.thresholds.All()[1].ID
	if rows[0].ThresholdVersionID != v1 || rows[1].ThresholdVersionID != v1 || rows[2].ThresholdVersionID != v2 {
		t.Fatalf("versiones usadas = %d,%d,%d; se esperaba %d,%d,%d",
			rows[0].ThresholdVersionID, rows[1].ThresholdVersionID, rows[2].ThresholdVersionID, v1, v1, v2)
	}
	if rows[1].RawLevel != entities.LevelAmarillo || rows[2].RawLevel != entities.LevelNormal {
		t.Fatalf("raw = %s / %s: el cambio de umbral aplica desde su valid_from", rows[1].RawLevel, rows[2].RawLevel)
	}
	// Las horas pasadas no se reinterpretan.
	if rows[1].Level != entities.LevelAmarillo || !rows[2].InEpisode || rows[2].Rule != entities.RuleHold {
		t.Fatalf("filas = %+v", rows)
	}
}

func TestEvaluateIndicator_PointWithoutVersionGetsFirstUseVersion(t *testing.T) {
	f := newFixture()

	r, err := f.svc.EvaluateIndicator(context.Background(), calculated(pointB, hour(0), 0.5))
	if err != nil || r.Outcome != services.OutcomeNoThresholds {
		t.Fatalf("outcome = %s, err = %v; se esperaba NO_THRESHOLDS", r.Outcome, err)
	}

	history, _ := f.thresholds.GetHistory(context.Background(), pointB, leak)
	if len(history) != 1 || !history[0].ValidFrom.Equal(now) || history[0].Reason != entities.FirstUseSeedReason {
		t.Fatalf("versión de primer uso = %+v", history)
	}
	if len(f.evaluations.Rows(pointB)) != 0 {
		t.Fatal("una hora anterior a la primera versión no se interpreta")
	}
}

func TestEvaluateIndicator_PublishFailureNeverFailsTheEvaluation(t *testing.T) {
	f := newFixture()
	f.publisher.err = errors.New("bus caído")

	results := f.feed(t, 0.20, 0.20)
	if results[1].Outcome != services.OutcomeSaved || results[1].Published || len(f.evaluations.Rows(pointA)) != 2 {
		t.Fatalf("resultado = %+v", results[1])
	}
}

func TestEvaluateIndicator_SaveFailureReturnsErrorAndDoesNotPublish(t *testing.T) {
	f := newFixture()
	f.feed(t, 0.20)
	f.evaluations.FailSave = errors.New("db caída")

	_, err := f.svc.EvaluateIndicator(context.Background(), calculated(pointA, hour(1), 0.20))
	if !errors.Is(err, domain.ErrPersistingRuleEvaluations) {
		t.Fatalf("err = %v, se esperaba ErrPersistingRuleEvaluations", err)
	}
	if len(f.publisher.exceeded) != 0 {
		t.Fatal("sin fila no se publica")
	}
}

func TestEvaluateIndicatorWithoutPublishing_SameRowsNoEvents(t *testing.T) {
	values := []float64{0.12, 0.19, 0.20, 0.30, 0.17, 0.15, 0.14}

	published := newFixture()
	published.feed(t, values...)

	silent := newFixture()
	for n, i := range values {
		if _, err := silent.svc.EvaluateIndicatorWithoutPublishing(context.Background(), calculated(pointA, hour(n), i)); err != nil {
			t.Fatal(err)
		}
	}

	a, b := published.evaluations.Rows(pointA), silent.evaluations.Rows(pointA)
	for n := range a {
		a[n].CreatedAt, b[n].CreatedAt = time.Time{}, time.Time{}
		if a[n] != b[n] {
			t.Fatalf("hora %d: %+v != %+v (debe ser exactamente la misma lógica)", n, a[n], b[n])
		}
	}
	if len(silent.publisher.exceeded)+len(silent.publisher.cleared) != 0 {
		t.Fatal("el modo sin publicación nunca publica")
	}
}
