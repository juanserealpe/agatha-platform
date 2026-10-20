package services_test

import (
	"context"
	"errors"
	"reflect"
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
	events []contract.LevelEvaluated
	err    error
}

func (p *fakePublisher) PublishLevelEvaluated(_ context.Context, e contract.LevelEvaluated) error {
	p.events = append(p.events, e)
	return p.err
}

func noCalculable(pointID int64, at time.Time) services.IndicatorEvaluatedNotice {
	return services.IndicatorEvaluatedNotice{
		MeasuringPointID: pointID, WindowHours: 24, EvaluatedAt: at,
		Status: "NO_CALCULABLE", NoCalcReason: "LOW_COVERAGE", Trigger: "RECONCILER",
	}
}

func f64(v float64) *float64 { return &v }

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

func TestEvaluateIndicator_IgnoresWindow6h(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	w6 := calculated(pointA, hour(0), 0.9)
	w6.WindowHours = 6
	w6NoCalc := noCalculable(pointA, hour(0))
	w6NoCalc.WindowHours = 6
	withoutI := calculated(pointA, hour(0), 0)
	withoutI.I = nil

	for _, n := range []services.IndicatorEvaluatedNotice{w6, w6NoCalc, withoutI} {
		r, err := f.svc.EvaluateIndicator(ctx, n)
		if err != nil || r.Outcome != services.OutcomeIgnored {
			t.Fatalf("outcome = %s, err = %v; se esperaba IGNORED", r.Outcome, err)
		}
	}
	if rows := f.evaluations.Rows(pointA); len(rows) != 0 {
		t.Fatalf("filas = %d, no debía crearse ninguna", len(rows))
	}
	if len(f.publisher.events) != 0 {
		t.Fatal("una ventana ignorada no publica")
	}
}

// RE-02: la hora NO_CALCULABLE de 24 h se reenvía sin crear fila.
func TestEvaluateIndicator_ForwardsNoCalculableWithoutRow(t *testing.T) {
	f := newFixture()

	r, err := f.svc.EvaluateIndicator(context.Background(), noCalculable(pointA, hour(0)))
	if err != nil || r.Outcome != services.OutcomeNoCalculable || !r.Published {
		t.Fatalf("resultado = %+v, err = %v; se esperaba NO_CALCULABLE publicado", r, err)
	}
	if rows := f.evaluations.Rows(pointA); len(rows) != 0 {
		t.Fatalf("filas = %d: NO_CALCULABLE no genera fila", len(rows))
	}

	want := contract.LevelEvaluated{
		MeasuringPointID: pointA,
		EventType:        "FUGA",
		EvaluatedAt:      hour(0),
		Status:           "NO_CALCULABLE",
		NoCalcReason:     "LOW_COVERAGE",
		OccurredAt:       now,
		SchemaVersion:    contract.SchemaVersion,
	}
	if len(f.publisher.events) != 1 || !reflect.DeepEqual(f.publisher.events[0], want) {
		t.Fatalf("eventos = %+v\nse esperaba    %+v", f.publisher.events, want)
	}
}

func TestEvaluateIndicator_NoCalculableIsNotForwardedWhenLateOrInBackfill(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	f.feed(t, 0.10, 0.10, 0.10) // horas 0..2
	published := len(f.publisher.events)

	// La hora ya tiene fila (CALCULATED): no se reenvía.
	r, err := f.svc.EvaluateIndicator(ctx, noCalculable(pointA, hour(1)))
	if err != nil || r.Outcome != services.OutcomeDuplicate || r.Published {
		t.Fatalf("hora ya interpretada: %+v, err = %v", r, err)
	}

	// Sin fila propia pero con horas posteriores: tardía, no se reenvía.
	pointID := pointA
	if _, err := f.evaluations.DeleteRange(ctx, &pointID, leak, hour(1), hour(2)); err != nil {
		t.Fatal(err)
	}
	r, err = f.svc.EvaluateIndicator(ctx, noCalculable(pointA, hour(1)))
	if err != nil || r.Outcome != services.OutcomeNoCalculable || !r.Late || r.Published {
		t.Fatalf("tardía: %+v, err = %v; se esperaba NO_CALCULABLE tardía sin publicar", r, err)
	}

	// El modo sin publicación (backfill) nunca reenvía.
	r, err = f.svc.EvaluateIndicatorWithoutPublishing(ctx, noCalculable(pointA, hour(3)))
	if err != nil || r.Published {
		t.Fatalf("backfill: %+v, err = %v", r, err)
	}
	if len(f.publisher.events) != published {
		t.Fatalf("eventos = %d, se esperaban %d", len(f.publisher.events), published)
	}
}

// RE-01/RE-03/RE-04/RE-05: una level_evaluated por CADA hora calculada,
// incluidas las de nivel NORMAL y las de histéresis.
func TestEvaluateIndicator_PublishesLevelEvaluatedEveryCalculatedHour(t *testing.T) {
	f := newFixture()

	// NORMAL, entrada pendiente, abre, sube, histéresis, salida 1/2, cierra, NORMAL.
	results := f.feed(t, 0.12, 0.19, 0.20, 0.30, 0.17, 0.15, 0.14, 0.10)

	rows := f.evaluations.Rows(pointA)
	if len(rows) != 8 || len(f.publisher.events) != 8 {
		t.Fatalf("filas = %d, eventos = %d; se esperaban 8 y 8", len(rows), len(f.publisher.events))
	}

	type hourWant struct{ raw, computed, episode, event string }
	wants := []hourWant{
		{"NORMAL", "NORMAL", "NORMAL", "NONE"},
		{"AMARILLO", "AMARILLO", "NORMAL", "NONE"}, // racha de entrada pendiente
		{"AMARILLO", "AMARILLO", "AMARILLO", "OPENED"},
		{"NARANJA", "NARANJA", "NARANJA", "NONE"},
		{"NORMAL", "NORMAL", "NARANJA", "NONE"}, // HOLD: computed no arrastra la histéresis
		{"NORMAL", "NORMAL", "NARANJA", "NONE"},
		{"NORMAL", "NORMAL", "NORMAL", "CLEARED"},
		{"NORMAL", "NORMAL", "NORMAL", "NONE"},
	}
	for n, e := range f.publisher.events {
		w := wants[n]
		if e.RawLevel != w.raw || e.ComputedLevel != w.computed || e.EpisodeLevel != w.episode || e.EpisodeEvent != w.event {
			t.Errorf("hora %d: raw/computed/episode/event = %s/%s/%s/%s; se esperaba %s/%s/%s/%s",
				n, e.RawLevel, e.ComputedLevel, e.EpisodeLevel, e.EpisodeEvent, w.raw, w.computed, w.episode, w.event)
		}
		if !results[n].Published || e.RuleEvaluationID != rows[n].ID || !e.EvaluatedAt.Equal(hour(n)) {
			t.Errorf("hora %d: published = %v, evento = %+v", n, results[n].Published, e)
		}
		if string(rows[n].EpisodeEvent) != w.event || string(rows[n].ComputedLevel) != w.computed {
			t.Errorf("hora %d: la fila guarda %s/%s", n, rows[n].ComputedLevel, rows[n].EpisodeEvent)
		}
	}

	want := contract.LevelEvaluated{
		RuleEvaluationID: rows[2].ID,
		MeasuringPointID: pointA,
		EventType:        "FUGA",
		EvaluatedAt:      hour(2),
		Status:           "CALCULATED",
		IndicatorValue:   f64(0.20),
		RawLevel:         "AMARILLO",
		ComputedLevel:    "AMARILLO",
		EpisodeLevel:     "AMARILLO",
		InEpisode:        true,
		EpisodeEvent:     "OPENED",
		Reasons: []contract.Reason{
			{Code: "LEAK_THRESHOLD", Level: "AMARILLO", Data: contract.ReasonData{I: 0.20, Threshold: 0.18}},
		},
		Thresholds:         contract.LevelCuts{Amarillo: 0.18, Naranja: 0.25, Rojo: 0.35},
		Exit:               contract.LevelCuts{Amarillo: 0.16, Naranja: 0.23, Rojo: 0.33},
		ThresholdVersionID: rows[2].ThresholdVersionID,
		HoursAbove:         2,
		OccurredAt:         now,
		SchemaVersion:      contract.SchemaVersion,
	}
	if got := f.publisher.events[2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("apertura = %+v\nse esperaba  %+v", got, want)
	}

	// Hora NORMAL fuera de episodio: también se publica, sin motivos.
	if e := f.publisher.events[0]; e.InEpisode || len(e.Reasons) != 0 || e.HoursAbove != 0 || e.Status != "CALCULATED" {
		t.Fatalf("hora NORMAL = %+v", e)
	}
	if e := f.publisher.events[6]; e.InEpisode || e.HoursAbove != 6 {
		t.Fatalf("cierre = %+v: hours_above es la duración del episodio, incluida la hora de cierre", e)
	}
}

// RE-05: la persistencia llega con su racha y el nivel desde el que se contó.
func TestEvaluateIndicator_PublishesPersistenceReason(t *testing.T) {
	f := newFixture()
	values := make([]float64, 12)
	for n := range values {
		values[n] = 0.22
	}
	f.feed(t, values...)

	last := f.publisher.events[len(f.publisher.events)-1]
	want := []contract.Reason{
		{Code: "LEAK_THRESHOLD", Level: "AMARILLO", Data: contract.ReasonData{I: 0.22, Threshold: 0.18}},
		{Code: "LEAK_PERSISTENCE", Level: "NARANJA", Data: contract.ReasonData{Hours: 12, From: "AMARILLO"}},
	}
	if last.RawLevel != "AMARILLO" || last.ComputedLevel != "NARANJA" || !reflect.DeepEqual(last.Reasons, want) {
		t.Fatalf("evento = %+v", last)
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
	if len(f.publisher.events) != 2 {
		t.Fatalf("eventos = %d: el duplicado no se republica", len(f.publisher.events))
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
	if len(rows) != 3 || rows[1].EpisodeEvent != entities.EpisodeEventOpened {
		t.Fatalf("filas = %+v: la hora tardía se interpreta igual (la 4 abre el episodio)", rows)
	}
	if len(f.publisher.events) != 1 || !f.publisher.events[0].EvaluatedAt.Equal(hour(5)) {
		t.Fatalf("eventos = %+v: solo se publica la hora 5 (las tardías no)", f.publisher.events)
	}
}

func TestEvaluateIndicator_NoCalculableGapPausesTheRun(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	steps := []services.IndicatorEvaluatedNotice{
		calculated(pointA, hour(0), 0.19),
		noCalculable(pointA, hour(1)),
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
	if len(f.evaluations.Rows(pointB)) != 0 || len(f.publisher.events) != 0 {
		t.Fatal("una hora anterior a la primera versión no se interpreta ni se publica")
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
	if len(f.publisher.events) != 1 {
		t.Fatalf("eventos = %d: sin fila no se publica", len(f.publisher.events))
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
		if !reflect.DeepEqual(a[n], b[n]) {
			t.Fatalf("hora %d: %+v != %+v (debe ser exactamente la misma lógica)", n, a[n], b[n])
		}
	}
	if len(silent.publisher.events) != 0 {
		t.Fatal("el modo sin publicación nunca publica")
	}
}
