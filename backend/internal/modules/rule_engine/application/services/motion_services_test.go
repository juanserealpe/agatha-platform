package services_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
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

// assertMatchesExpected compara las filas guardadas con motion_expected.csv.
func assertMatchesExpected(t *testing.T, rows []entities.MotionEvaluation) {
	t.Helper()
	want := readMotionCSV(t, "motion_expected.csv")
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
			"computed_level": string(g.ComputedLevel),
		}
		for k, v := range got {
			if v != w[k] {
				t.Fatalf("%s %s = %q, se esperaba %q", w["evaluated_at"], k, v, w[k])
			}
		}
		if gc, wc := reasonCodes(g), expectedCodes(t, w["reasons"]); len(gc) != len(wc) || (len(gc) > 0 && !equalStrings(gc, wc)) {
			t.Fatalf("%s motivos = %v, se esperaban %v", w["evaluated_at"], gc, wc)
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
		!e.InEpisode, e.EpisodeEvent != "NONE", e.SchemaVersion != 1, e.MotionEvaluationID != last.ID:
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
		PointID: pilot, Patch: entities.MotionPatch{"abrupt_deg": 0.05, "alerts_enabled": false},
		Reason: "calibración", CreatedBy: "ana",
	})
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	want := entities.DefaultMotionParams()
	want.AbruptDeg, want.AlertsEnabled = 0.05, false
	if v.Version != 2 || v.Params != want || v.PreviousVersionID == nil || !v.ValidFrom.Equal(f.clock.Now()) {
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

	// Subir abrupt_deg a 0,05° quita los dos cambios bruscos de octubre (+0,04° y +0,045°).
	higher, err := sim.Simulate(ctx, services.MotionSimulateCommand{PointID: pilot, Patch: entities.MotionPatch{"abrupt_deg": 0.05}, From: from, To: to})
	if err != nil {
		t.Fatal(err)
	}
	if len(higher.Episodes) != 2 || higher.RemovedEpisodes != 2 || higher.NewEpisodes != 0 {
		t.Fatalf("abrupt_deg 0,05: %d episodios, nuevos %d, quitados %d", len(higher.Episodes), higher.NewEpisodes, higher.RemovedEpisodes)
	}
	if len(f.evaluations.All(pilot)) != 0 {
		t.Fatal("la simulación escribió filas")
	}
}

var _ ports.RawMeasurementSource = (*fakeRawSource)(nil)
