package services_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
)

const motionTestdata = "../../domain/services/testdata/motion/"

var (
	pilot = entities.MotionPilotPointID
	// motionFirst es la primera medición de motion_input.csv; motionEnd, la
	// última hora de motion_expected.csv.
	motionFirst = time.Date(2026, 7, 8, 5, 15, 42, 0, time.UTC)
	motionEnd   = time.Date(2026, 11, 17, 19, 0, 0, 0, time.UTC)
)

// ---- fakes ----

type motionClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *motionClock) Now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *motionClock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

type fakeRawSource struct {
	records []entities.RawRecord
	devices []int64
	reads   int
}

func (s *fakeRawSource) ListRecords(_ context.Context, pointID int64, from, to time.Time) ([]entities.RawRecord, error) {
	s.reads++
	if pointID != pilot {
		return nil, nil
	}
	i := sort.Search(len(s.records), func(k int) bool { return !s.records[k].Timestamp.Before(from) })
	j := sort.Search(len(s.records), func(k int) bool { return !s.records[k].Timestamp.Before(to) })
	return append([]entities.RawRecord(nil), s.records[i:j]...), nil
}

func (s *fakeRawSource) FirstMeasurementAt(_ context.Context, pointID int64) (*time.Time, error) {
	if pointID != pilot || len(s.records) == 0 {
		return nil, nil
	}
	t := s.records[0].Timestamp
	return &t, nil
}

func (s *fakeRawSource) ListPoints(context.Context) ([]int64, error) { return s.devices, nil }

type fakeMotionPublisher struct {
	events []contract.MotionEvaluated
}

func (p *fakeMotionPublisher) PublishMotionEvaluated(_ context.Context, e contract.MotionEvaluated) error {
	p.events = append(p.events, e)
	return nil
}

var (
	motionRecordsOnce sync.Once
	motionRecords     []entities.RawRecord
)

func loadMotionRecords(t *testing.T) []entities.RawRecord {
	t.Helper()
	motionRecordsOnce.Do(func() {
		for _, r := range readMotionCSV(t, "motion_input.csv") {
			ts, err := time.Parse(time.RFC3339, r["timestamp"])
			if err != nil {
				t.Fatalf("timestamp %q: %v", r["timestamp"], err)
			}
			motionRecords = append(motionRecords, entities.RawRecord{
				Timestamp: ts, Pitch: optF(r["pitch"]), Roll: optF(r["roll"]), Deriva: optF(r["deriva"]), Temp: optF(r["temp"]),
			})
		}
		sort.Slice(motionRecords, func(i, j int) bool { return motionRecords[i].Timestamp.Before(motionRecords[j].Timestamp) })
	})
	return motionRecords
}

func readMotionCSV(t *testing.T, name string) []map[string]string {
	t.Helper()
	f, err := os.Open(motionTestdata + name)
	if err != nil {
		t.Fatalf("abrir %s: %v", name, err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("leer %s: %v", name, err)
	}
	out := make([]map[string]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		m := map[string]string{}
		for i, h := range recs[0] {
			m[h] = rec[i]
		}
		out = append(out, m)
	}
	return out
}

func optF(s string) *float64 {
	if s == "" {
		return nil
	}
	v, _ := strconv.ParseFloat(s, 64)
	return &v
}

// ---- fixture ----

type motionFixture struct {
	clock       *motionClock
	source      *fakeRawSource
	thresholds  *memory.MotionThresholdRepository
	evaluations *memory.MotionEvaluationRepository
	publisher   *fakeMotionPublisher
	thresholdUC *services.MotionThresholdService
	evaluator   *services.MotionEvaluationService
	backfill    *services.MotionBackfillService
}

func pilotVersion(p entities.MotionParams) entities.MotionThresholdVersion {
	return entities.MotionThresholdVersion{
		MeasuringPointID: pilot, Version: 1, Params: p, ValidFrom: motionFirst,
		Reason: entities.SeedReason, CreatedBy: entities.SystemCreatedBy,
	}
}

func newMotionFixture(t *testing.T, env string, versions ...entities.MotionThresholdVersion) *motionFixture {
	t.Helper()
	if len(versions) == 0 {
		versions = []entities.MotionThresholdVersion{pilotVersion(entities.DefaultMotionParams())}
	}
	f := &motionFixture{
		clock:       &motionClock{t: motionEnd.Add(30 * time.Minute)},
		source:      &fakeRawSource{records: loadMotionRecords(t), devices: []int64{pilot}},
		thresholds:  memory.NewMotionThresholdRepository(versions...),
		evaluations: memory.NewMotionEvaluationRepository(),
		publisher:   &fakeMotionPublisher{},
	}
	f.thresholdUC = services.NewMotionThresholdService(f.thresholds, f.clock, testLogger())
	f.evaluator = services.NewMotionEvaluationService(f.thresholds, f.evaluations, f.thresholdUC, f.source, f.publisher, f.clock, testLogger())
	f.backfill = services.NewMotionBackfillService(f.evaluator, services.MotionBackfillConfig{Environment: env})
	return f
}

func reasonCodes(e entities.MotionEvaluation) []string {
	out := make([]string, 0, len(e.Reasons))
	for _, r := range e.Reasons {
		out = append(out, string(r.Code))
	}
	sort.Strings(out)
	return out
}

func expectedCodes(t *testing.T, raw string) []string {
	t.Helper()
	var rs []struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(raw), &rs); err != nil {
		t.Fatalf("reasons %q: %v", raw, err)
	}
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Code)
	}
	sort.Strings(out)
	return out
}

// assertMatchesExpected compara las filas guardadas con motion_v2_expected.csv
// (estado, nivel, tipo, motivos y el historial de las horas OPENED, que
// depende de los episodios anteriores leídos con ListEpisodes).
func assertMatchesExpected(t *testing.T, rows []entities.MotionEvaluation) {
	t.Helper()
	want := readMotionCSV(t, "../motion_v2/motion_v2_expected.csv")
	if len(rows) != len(want) {
		t.Fatalf("filas = %d, se esperaban %d", len(rows), len(want))
	}
	bstr := map[bool]string{true: "True", false: "False"}
	for i, w := range want {
		g := rows[i]
		got := map[string]string{
			"evaluated_at": g.EvaluatedAt.Format(time.RFC3339), "status": string(g.Status),
			"motion_state": string(g.MotionState), "temp_state": string(g.State.TempState),
			"in_episode": bstr[g.State.InEpisode], "episode_event": string(g.EpisodeEvent),
			"computed_level": string(g.ComputedLevel), "table_level": string(g.TableLevel),
			"episode_kind": string(g.EpisodeKind),
		}
		for k, v := range got {
			if v != w[k] {
				t.Fatalf("%s %s = %q, se esperaba %q", w["evaluated_at"], k, v, w[k])
			}
		}
		if gc, wc := reasonCodes(g), expectedCodes(t, w["reasons"]); len(gc) != len(wc) || (len(gc) > 0 && !equalStrings(gc, wc)) {
			t.Fatalf("%s motivos = %v, se esperaban %v", w["evaluated_at"], gc, wc)
		}
		if w["history"] != "" || g.History != nil {
			var wh, gh any
			_ = json.Unmarshal([]byte(w["history"]), &wh)
			b, _ := json.Marshal(g.History)
			_ = json.Unmarshal(b, &gh)
			if !reflect.DeepEqual(wh, gh) {
				t.Fatalf("%s history:\n got  %s\n want %s", w["evaluated_at"], b, w["history"])
			}
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- backfill ----

func TestMotionBackfill_ReproducesReferenceWithoutPublishing(t *testing.T) {
	f := newMotionFixture(t, "")
	res, err := f.backfill.Backfill(context.Background(), services.MotionBackfillCommand{PointID: pilot})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	rows := f.evaluations.All(pilot)
	assertMatchesExpected(t, rows)

	if len(f.publisher.events) != 0 {
		t.Fatalf("el backfill publicó %d eventos", len(f.publisher.events))
	}
	for _, r := range rows {
		if r.Trigger != entities.MotionTriggerBackfill || r.ThresholdVersionID == 0 || r.AlgorithmVersion != entities.MotionAlgorithmVersion {
			t.Fatalf("%v: trigger %s, versión %d, algoritmo %q", r.EvaluatedAt, r.Trigger, r.ThresholdVersionID, r.AlgorithmVersion)
		}
	}
	p := res.Points[0]
	if p.Saved != 3182 || p.NoCalculable != 264 || p.Opened != 4 || p.Cleared != 4 {
		t.Fatalf("resumen %+v", p)
	}
	// Una lectura de raw_measurements por tramo de 168 h, no una por hora.
	if f.source.reads > 3182/168+2 {
		t.Fatalf("lecturas de raw_measurements = %d", f.source.reads)
	}
}

func TestMotionBackfill_ResumesAndIsIdempotent(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	if _, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot, To: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("primer tramo: %v", err)
	}
	if last := f.evaluations.All(pilot); !last[len(last)-1].EvaluatedAt.Equal(time.Date(2026, 8, 25, 23, 0, 0, 0, time.UTC)) {
		t.Fatalf("última hora del primer tramo = %v", last[len(last)-1].EvaluatedAt)
	}
	if _, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot}); err != nil {
		t.Fatalf("retomar: %v", err)
	}
	assertMatchesExpected(t, f.evaluations.All(pilot))

	again, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot})
	if err != nil || again.Points[0].Saved != 0 {
		t.Fatalf("repetir guardó %d filas (err %v)", again.Points[0].Saved, err)
	}
}

func TestMotionBackfill_RebuildRecomputesTheRange(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	if _, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot}); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	res, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot, From: from, Rebuild: true})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if res.Deleted == 0 || int64(res.Points[0].Saved) != res.Deleted {
		t.Fatalf("borradas %d, recalculadas %d", res.Deleted, res.Points[0].Saved)
	}
	assertMatchesExpected(t, f.evaluations.All(pilot))
}

func TestMotionBackfill_RebuildForbiddenInProduction(t *testing.T) {
	f := newMotionFixture(t, "Production")
	_, err := f.backfill.Backfill(context.Background(), services.MotionBackfillCommand{PointID: pilot, Rebuild: true})
	if !errors.Is(err, domain.ErrRebuildForbidden) {
		t.Fatalf("err = %v, se esperaba ErrRebuildForbidden", err)
	}
	if len(f.evaluations.All(pilot)) != 0 {
		t.Fatal("se escribieron filas")
	}
}

// ---- en vivo ----

func TestMotionLive_CatchUpThenLiveAndPublishesEvent(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	if _, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot, To: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	f.clock.set(time.Date(2026, 8, 26, 3, 40, 5, 0, time.UTC))

	run := f.evaluator.EvaluateLive(ctx, pilot)
	if run.Err != nil || run.Saved != 28 || run.Published != 28 {
		t.Fatalf("corrida %+v", run)
	}
	rows := f.evaluations.All(pilot)
	last := rows[len(rows)-1]
	if !last.EvaluatedAt.Equal(time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC)) || last.Trigger != entities.MotionTriggerLive {
		t.Fatalf("última fila %v con trigger %s", last.EvaluatedAt, last.Trigger)
	}
	if prev := rows[len(rows)-2]; prev.Trigger != entities.MotionTriggerCatchUp {
		t.Fatalf("anterior con trigger %s, se esperaba CATCH_UP", prev.Trigger)
	}

	// El evento de la hora 26 ago 03:00Z (ejemplo de §6.3).
	e := f.publisher.events[len(f.publisher.events)-1]
	started := time.Date(2026, 8, 25, 11, 15, 42, 0, time.UTC)
	opened := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	switch {
	case e.EventType != "MOVIMIENTO", e.Status != "CALCULATED", e.MotionState != "DESPLAZADO",
		e.TempState != "TEMP_ESTABLE", e.TempMode != "MOVIMIENTO_Y_TEMPERATURA", e.ComputedLevel != "AMARILLO",
		!e.InEpisode, e.EpisodeEvent != "NONE", e.SchemaVersion != 2, e.MotionEvaluationID != last.ID,
		e.TableLevel != "AMARILLO", e.EpisodeKind != "SALTO", e.History != nil:
		t.Fatalf("evento %+v", e)
	case e.EpisodeOpenedAt == nil || !e.EpisodeOpenedAt.Equal(opened):
		t.Fatalf("episode_opened_at = %v", e.EpisodeOpenedAt)
	case e.Shift.StartedAt == nil || !e.Shift.StartedAt.Equal(started) || e.Shift.GapFrom != nil:
		t.Fatalf("shift = %+v", e.Shift)
	case e.Shift.Pitch == nil || e.Shift.Roll == nil:
		t.Fatalf("shift sin pitch/roll: %+v", e.Shift)
	case e.TempStable.From == nil || !e.TempStable.From.Equal(started) || e.TempStable.To != nil:
		t.Fatalf("temp_stable = %+v", e.TempStable)
	case len(e.Reasons) != 1 || e.Reasons[0].Code != "TEMP_STABILIZED" || e.Reasons[0].Data["temp_rate_max_6h"] != 0.025 ||
		e.Reasons[0].Data["temp_stable_from"] != "2026-08-25T11:15:42Z":
		t.Fatalf("motivos = %+v", e.Reasons)
	}
	// Nivel −3,11 / 25,56 menos la referencia −3,18 / 25,42 (motion_expected.csv).
	// El ejemplo de §6.3 dice 0,08 / 0,13: es un error del ejemplo.
	if *e.Shift.Pitch != 0.07 || *e.Shift.Roll != 0.14 {
		t.Fatalf("shift = %v / %v, se esperaba 0.07 / 0.14", *e.Shift.Pitch, *e.Shift.Roll)
	}

	// CA-8: repetir no crea filas ni eventos.
	again := f.evaluator.EvaluateLive(ctx, pilot)
	if again.Saved != 0 || again.Published != 0 || len(f.publisher.events) != 28 {
		t.Fatalf("repetir: %+v, eventos %d", again, len(f.publisher.events))
	}
}

func TestMotionLive_ShadowModeSavesRowsWithoutEvents(t *testing.T) {
	f := newMotionFixture(t, "", pilotVersion(entities.NewPointMotionParams()))
	ctx := context.Background()
	if _, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot, To: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	f.clock.set(time.Date(2026, 8, 29, 0, 40, 0, 0, time.UTC))
	run := f.evaluator.EvaluateLive(ctx, pilot)
	if run.Saved == 0 || run.Published != 0 || len(f.publisher.events) != 0 {
		t.Fatalf("modo sombra: %+v, eventos %d", run, len(f.publisher.events))
	}
	if run.Opened == 0 {
		t.Fatal("en modo sombra el episodio se calcula igual")
	}
	for _, r := range f.evaluations.All(pilot) {
		if r.State.TempState != "" {
			t.Fatalf("%v: temp_state %s con SOLO_MOVIMIENTO", r.EvaluatedAt, r.State.TempState)
		}
	}
}

func TestMotionLive_PointWithoutRowsIsSkipped(t *testing.T) {
	f := newMotionFixture(t, "")
	run := f.evaluator.EvaluateLive(context.Background(), pilot)
	if !run.WithoutHistory || run.Saved != 0 || run.Err != nil {
		t.Fatalf("corrida %+v", run)
	}
}

func TestMotionRunLive_NewDeviceGetsShadowVersion(t *testing.T) {
	f := newMotionFixture(t, "")
	f.source.devices = []int64{pilot, 999}
	runs, err := f.evaluator.RunLive(context.Background())
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs %+v, err %v", runs, err)
	}
	history, _ := f.thresholds.GetHistory(context.Background(), 999)
	if len(history) != 1 || history[0].Params.AlertsEnabled || history[0].Params.TempMode != entities.TempModeMotionOnly ||
		!history[0].ValidFrom.Equal(f.clock.Now()) {
		t.Fatalf("versión del punto nuevo %+v", history)
	}
	for _, r := range runs {
		if !r.WithoutHistory {
			t.Fatalf("punto %d sin filas no se salteó: %+v", r.PointID, r)
		}
	}
}

// ---- versiones ----

func TestMotionThresholds_CreateCopiesAndClosesTheOpenVersion(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	v, err := f.thresholdUC.CreateVersion(ctx, services.CreateMotionThresholdVersionCommand{
		PointID: pilot, Patch: entities.MotionPatch{"still_hours": 25, "alerts_enabled": false},
		Reason: "calibración", CreatedBy: "ana",
	})
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	want := entities.DefaultMotionParams()
	want.StillHours, want.AlertsEnabled = 25, false
	if v.Version != 2 || !reflect.DeepEqual(v.Params, want) || v.PreviousVersionID == nil || !v.ValidFrom.Equal(f.clock.Now()) {
		t.Fatalf("versión %+v", v)
	}
	history, _ := f.thresholdUC.GetHistory(ctx, pilot)
	if len(history) != 2 || history[0].ValidTo == nil || !history[0].ValidTo.Equal(v.ValidFrom) {
		t.Fatalf("historial %+v", history)
	}
}

func TestMotionThresholds_Validation(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	cases := []struct {
		name string
		cmd  services.CreateMotionThresholdVersionCommand
		want error
	}{
		{"sin autor", services.CreateMotionThresholdVersionCommand{PointID: pilot, Patch: entities.MotionPatch{}}, domain.ErrMissingAuthorship},
		{"retroactiva", services.CreateMotionThresholdVersionCommand{PointID: pilot, Patch: entities.MotionPatch{},
			ValidFrom: f.clock.Now().Add(-time.Hour), Reason: "r", CreatedBy: "c"}, domain.ErrValidFromInPast},
		{"salto mayor que desplazamiento", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"jump_deg": 0.05}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionJumpShift},
		{"temp_mode inválido", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"temp_mode": entities.TempMode("X")}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionTempMode},
		{"horas en cero", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"still_hours": 0}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionHoursRange},
		{"min_records_hour > records_per_hour", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"min_records_hour": 5}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionRecordsRange},
		{"parámetro desconocido", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"amarillo": 0.2}, Reason: "r", CreatedBy: "c"}, domain.ErrInvalidPatch},
		// CA-4 (§5.4).
		{"amarillo_deg <= 0,01", rulesCmd(rule(3, "ANY", 0.01, nil, nil)), domain.ErrMotionRuleAmarillo},
		{"naranja <= amarillo", rulesCmd(rule(3, "ANY", 0.04, f64(0.04), nil)), domain.ErrMotionRuleOrder},
		{"rojo <= naranja", rulesCmd(rule(3, "ANY", 0.04, f64(0.10), f64(0.10))), domain.ErrMotionRuleOrder},
		{"rojo <= amarillo sin naranja", rulesCmd(rule(1, "ANY", 0.04, nil, f64(0.03))), domain.ErrMotionRuleOrder},
		{"ventana repetida", rulesCmd(rule(3, "PITCH", 0.04, nil, nil), rule(3, "PITCH", 0.05, nil, nil)), domain.ErrMotionRuleDuplicate},
		{"ANY con PITCH", rulesCmd(rule(3, "ANY", 0.04, nil, nil), rule(3, "PITCH", 0.05, nil, nil)), domain.ErrMotionRuleDuplicate},
		{"ventana 0", rulesCmd(rule(0, "ANY", 0.04, nil, nil)), domain.ErrMotionRuleWindow},
		{"ventana 721", rulesCmd(rule(721, "ANY", 0.04, nil, nil)), domain.ErrMotionRuleWindow},
		{"eje inválido", rulesCmd(rule(3, "YAW", 0.04, nil, nil)), domain.ErrMotionRuleAxis},
		{"tabla vacía", rulesCmd(), domain.ErrMotionRulesEmpty},
		{"send_naranja inválido", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"send_naranja": "ON"}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionSendMode},
		{"closure_cap_level inválido", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"closure_cap_level": "NORMAL"}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionClosureCap},
		{"long_min_coverage 0", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"long_min_coverage": 0.0}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionFraction},
		{"history_step_share > 1", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"history_step_share": 1.5}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionFraction},
		{"abrupt_deg retirado", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"abrupt_deg": 0.05}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionAbruptRetired},
		{"abrupt_hours retirado", services.CreateMotionThresholdVersionCommand{PointID: pilot,
			Patch: entities.MotionPatch{"abrupt_hours": 4}, Reason: "r", CreatedBy: "c"}, domain.ErrMotionAbruptRetired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.thresholdUC.CreateVersion(ctx, tc.cmd); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, se esperaba %v", err, tc.want)
			}
		})
	}
	if h, _ := f.thresholdUC.GetHistory(ctx, pilot); len(h) != 1 {
		t.Fatalf("se crearon versiones inválidas: %d", len(h))
	}
}

func rule(w int, axis string, am float64, na, ro *float64) entities.MotionRule {
	return entities.MotionRule{WindowHours: w, Axis: axis, AmarilloDeg: am, NaranjaDeg: na, RojoDeg: ro}
}

func rulesCmd(rules ...entities.MotionRule) services.CreateMotionThresholdVersionCommand {
	if rules == nil {
		rules = []entities.MotionRule{}
	}
	return services.CreateMotionThresholdVersionCommand{PointID: pilot,
		Patch: entities.MotionPatch{entities.MotionRulesParam: rules}, Reason: "r", CreatedBy: "c"}
}

// CA-4: los cuatro avisos se guardan igual y vuelven en Warnings.
func TestMotionThresholds_Warnings(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		rules []entities.MotionRule
		want  []string
	}{
		{"tabla de §5.3", entities.DefaultMotionRules(), nil},
		{"lo normal no crece", []entities.MotionRule{rule(3, "ANY", 0.10, nil, nil), rule(168, "PITCH", 0.05, nil, nil)},
			[]string{entities.MotionWarnNormalNotIncreasing}},
		{"sin regla larga", []entities.MotionRule{rule(3, "ANY", 0.04, nil, nil)}, []string{entities.MotionWarnNoLongRule}},
		{"sin regla corta", []entities.MotionRule{rule(168, "ANY", 0.10, nil, nil)}, []string{entities.MotionWarnNoShortRule}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMotionFixture(t, "")
			v, err := f.thresholdUC.CreateVersion(ctx, rulesCmd(tc.rules...))
			if err != nil || !equalStrings(v.Warnings, tc.want) {
				t.Fatalf("warnings %v (err %v), se esperaba %v", v.Warnings, err, tc.want)
			}
			if h, _ := f.thresholdUC.GetHistory(ctx, pilot); len(h) != 2 {
				t.Fatalf("con avisos se guarda igual: %d versiones", len(h))
			}
		})
	}
	// WINDOW_LONGER_THAN_HISTORY: con 10 días de historia, la ventana de 30 días avisa.
	f := newMotionFixture(t, "")
	f.thresholdUC.WithMeasurementSource(f.source)
	f.clock.set(motionFirst.Add(10 * 24 * time.Hour))
	v, err := f.thresholdUC.CreateVersion(ctx, rulesCmd(entities.DefaultMotionRules()...))
	if err != nil || !equalStrings(v.Warnings, []string{entities.MotionWarnWindowLongerThanHistory}) {
		t.Fatalf("warnings %v (err %v)", v.Warnings, err)
	}
}

// CA-5: sin rules se copia la tabla; con rules se reemplaza completa; el
// historial devuelve la tabla de cada versión.
func TestMotionThresholds_RulesCopyAndReplace(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	v2, err := f.thresholdUC.CreateVersion(ctx, services.CreateMotionThresholdVersionCommand{PointID: pilot,
		Patch: entities.MotionPatch{"send_naranja": "ACTIVE"}, Reason: "r", CreatedBy: "c"})
	if err != nil || !reflect.DeepEqual(v2.Params.Rules, entities.DefaultMotionRules()) || v2.Params.SendNaranja != entities.MotionSendActive {
		t.Fatalf("sin rules: %+v (err %v)", v2.Params, err)
	}
	f.clock.set(f.clock.Now().Add(time.Hour))
	only := []entities.MotionRule{rule(3, "PITCH", 0.05, nil, nil), rule(168, "ANY", 0.12, nil, nil)}
	v3, err := f.thresholdUC.CreateVersion(ctx, rulesCmd(only...))
	if err != nil || !reflect.DeepEqual(v3.Params.Rules, only) || v3.Params.SendNaranja != entities.MotionSendActive {
		t.Fatalf("con rules: %+v (err %v)", v3.Params.Rules, err)
	}
	h, _ := f.thresholdUC.GetHistory(ctx, pilot)
	if len(h) != 3 || !reflect.DeepEqual(h[0].Params.Rules, entities.DefaultMotionRules()) ||
		!reflect.DeepEqual(h[1].Params.Rules, entities.DefaultMotionRules()) || !reflect.DeepEqual(h[2].Params.Rules, only) {
		t.Fatalf("historial de tablas: %+v", h)
	}
}

// ---- simulación ----

func TestMotionSimulation_ComparesWithCurrentVersion(t *testing.T) {
	f := newMotionFixture(t, "")
	sim := services.NewMotionSimulationService(f.thresholds, f.source, f.clock, testLogger())
	ctx := context.Background()
	from, to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	same, err := sim.Simulate(ctx, services.MotionSimulateCommand{PointID: pilot, Patch: entities.MotionPatch{}, From: from, To: to})
	if err != nil {
		t.Fatalf("simular: %v", err)
	}
	if len(same.Episodes) != 4 || same.NewEpisodes != 0 || same.RemovedEpisodes != 0 {
		t.Fatalf("sin cambios: %d episodios, nuevos %d, quitados %d", len(same.Episodes), same.NewEpisodes, same.RemovedEpisodes)
	}
	aug := same.Episodes[0]
	if aug.TempStableFrom == nil || aug.TempStableTo == nil || aug.ClearedAt == nil {
		t.Fatalf("episodio de agosto %+v", aug)
	}

	// CA-6: tipos y niveles de §11.1.
	for i, k := range []entities.MotionEpisodeKind{entities.MotionEpisodeSalto, entities.MotionEpisodeRapido,
		entities.MotionEpisodeRapido, entities.MotionEpisodeHueco} {
		if e := same.Episodes[i]; e.Kind != k || e.TableLevel != entities.LevelAmarillo || e.SentLevel != entities.LevelAmarillo {
			t.Fatalf("episodio %d: %s %s/%s, se esperaba %s AMARILLO/AMARILLO", i, e.Kind, e.TableLevel, e.SentLevel, k)
		}
	}

	// CA-6: con amarillo_deg = 0,03 en la fila de 3 h aparecen episodios nuevos.
	rules := entities.DefaultMotionRules()
	rules[1].AmarilloDeg = 0.03
	lower, err := sim.Simulate(ctx, services.MotionSimulateCommand{PointID: pilot,
		Patch: entities.MotionPatch{entities.MotionRulesParam: rules}, From: from, To: to})
	if err != nil {
		t.Fatal(err)
	}
	if lower.NewEpisodes == 0 || len(lower.Episodes) <= 4 || len(lower.CurrentEpisodes) != 4 {
		t.Fatalf("amarillo 0,03 en 3 h: %d episodios, nuevos %d, vigentes %d", len(lower.Episodes), lower.NewEpisodes, len(lower.CurrentEpisodes))
	}
	if len(f.evaluations.All(pilot)) != 0 {
		t.Fatal("la simulación escribió filas")
	}
}

var _ ports.RawMeasurementSource = (*fakeRawSource)(nil)

// §8 (P2): observed lee motion_evaluations (ListRange) y, con la tabla
// vigente, da los máximos de §2 en los últimos 90 días; days fuera de 1..365 es 400.
func TestMotionObserved_LastNinetyDays(t *testing.T) {
	f := newMotionFixture(t, "")
	ctx := context.Background()
	if _, err := f.backfill.Backfill(ctx, services.MotionBackfillCommand{PointID: pilot}); err != nil {
		t.Fatal(err)
	}
	obs := services.NewMotionObservedService(f.thresholds, f.evaluations, f.clock, testLogger())
	res, err := obs.Observed(ctx, pilot, 90)
	if err != nil || len(res.Rows) != 10 || !res.To.Equal(motionEnd) {
		t.Fatalf("observed %+v (err %v)", res, err)
	}
	if r := res.Rows[len(res.Rows)-1]; r.WindowHours != 720 || r.Axis != "roll" || r.Max == nil || *r.Max != 0.095 {
		t.Fatalf("30 días roll %+v", r)
	}
	for _, days := range []int{0, 366} {
		if _, err := obs.Observed(ctx, pilot, days); !errors.Is(err, domain.ErrMotionObservedDays) {
			t.Fatalf("days %d: %v", days, err)
		}
	}
}
