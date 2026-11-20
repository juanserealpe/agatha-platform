package bootstrap

import (
	"context"
	"encoding/csv"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	alertPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	alertServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	alertEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	alertMappers "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/mappers"
	alertMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
	alertModels "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
	ruleEnginePorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	ruleEngineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	ruleEngineEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	ruleEngineMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// §12.3: el backfill intercalado (FUGA + MOVIMIENTO) del MP-11556 reproduce
// las 9 alertas esperadas. El movimiento es el real: MotionAssess sobre
// motion_input.csv, traducido con el mismo camino que el evento en vivo
// (MotionEvaluatedFromRow → toMotionNotice). La FUGA:
//   - por defecto, episodios sintéticos con los inicios, fines y niveles de la
//     tabla (lo que se prueba es el acople con el movimiento real);
//   - con testdata/rule_evaluations_11556.csv (export de rule_evaluations del
//     punto, ver README), el histórico real mapeado con ToLevelNotice.

const motionTestdataDir = "../modules/rule_engine/domain/services/testdata/motion/"

func utc(month time.Month, day, hour int) time.Time {
	return time.Date(2026, month, day, hour, 0, 0, 0, time.UTC)
}

// ---- movimiento real ----

type csvRawSource struct {
	records []ruleEngineEntities.RawRecord
}

func (s csvRawSource) ListRecords(_ context.Context, _ int64, from, to time.Time) ([]ruleEngineEntities.RawRecord, error) {
	i := sort.Search(len(s.records), func(k int) bool { return !s.records[k].Timestamp.Before(from) })
	j := sort.Search(len(s.records), func(k int) bool { return !s.records[k].Timestamp.Before(to) })
	return s.records[i:j], nil
}
func (s csvRawSource) FirstMeasurementAt(context.Context, int64) (*time.Time, error) {
	t := s.records[0].Timestamp
	return &t, nil
}
func (csvRawSource) ListPoints(context.Context) ([]int64, error) {
	return []int64{ruleEngineEntities.MotionPilotPointID}, nil
}

func readCSVFile(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func optFloatField(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// referenceMotionNotices corre el Rule Engine sobre motion_input.csv y traduce cada fila.
func referenceMotionNotices(t *testing.T) []alertEntities.MotionNotice {
	t.Helper()
	var records []ruleEngineEntities.RawRecord
	for _, r := range readCSVFile(t, motionTestdataDir+"motion_input.csv")[1:] {
		ts, _ := time.Parse(time.RFC3339, r[0])
		records = append(records, ruleEngineEntities.RawRecord{Timestamp: ts, Pitch: optFloatField(r[1]), Roll: optFloatField(r[2]),
			Deriva: optFloatField(r[3]), Temp: optFloatField(r[4])})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Timestamp.Before(records[j].Timestamp) })

	now := time.Date(2026, 11, 17, 19, 30, 0, 0, time.UTC)
	clock := ruleEnginePorts.ClockFunc(func() time.Time { return now })
	log := logger.New("MotionAlertsTest", "Error")
	params := ruleEngineEntities.DefaultMotionParams()
	thresholds := ruleEngineMemory.NewMotionThresholdRepository(ruleEngineEntities.MotionThresholdVersion{
		MeasuringPointID: ruleEngineEntities.MotionPilotPointID, Version: 1, Params: params, ValidFrom: records[0].Timestamp, Reason: "r", CreatedBy: "c",
	})
	evaluations := ruleEngineMemory.NewMotionEvaluationRepository()
	evaluator := ruleEngineServices.NewMotionEvaluationService(thresholds, evaluations, nil, csvRawSource{records}, nil, clock, log)
	if _, err := ruleEngineServices.NewMotionBackfillService(evaluator, ruleEngineServices.MotionBackfillConfig{}).
		Backfill(context.Background(), ruleEngineServices.MotionBackfillCommand{PointID: ruleEngineEntities.MotionPilotPointID}); err != nil {
		t.Fatal(err)
	}
	rows := evaluations.All(ruleEngineEntities.MotionPilotPointID)
	out := make([]alertEntities.MotionNotice, len(rows))
	for i, row := range rows {
		out[i] = toMotionNotice(ruleEngineServices.MotionEvaluatedFromRow(row, params, now))
	}
	return out
}

// ---- FUGA ----

// leakEpisode es un episodio sintético: started (primera hora sobre el
// corte), ended (CLEARED) y la forma de los niveles.
type leakEpisode struct {
	started, ended time.Time
	shape          func(k int, hours int) alertEntities.Level // nivel de la hora k del episodio (k ≥ 1)
}

func amarillo(int, int) alertEntities.Level { return alertEntities.LevelAmarillo }

// rojo sube a Rojo y, 8 h antes del fin, baja a Naranja (ROJO_HELD dentro del episodio).
func rojo(k, hours int) alertEntities.Level {
	switch {
	case k == 1:
		return alertEntities.LevelAmarillo
	case k == 2:
		return alertEntities.LevelNaranja
	case k >= hours-8:
		return alertEntities.LevelNaranja
	default:
		return alertEntities.LevelRojo
	}
}

func naranja(k, _ int) alertEntities.Level {
	if k == 1 {
		return alertEntities.LevelAmarillo
	}
	return alertEntities.LevelNaranja
}

var leakValue = map[alertEntities.Level]float64{
	alertEntities.LevelNormal: 0.08, alertEntities.LevelAmarillo: 0.19, alertEntities.LevelNaranja: 0.28, alertEntities.LevelRojo: 0.40,
}

func leakHour(at time.Time, raw, computed alertEntities.Level, event alertEntities.EpisodeEvent, inEpisode bool, hoursAbove int) alertEntities.LevelNotice {
	v := leakValue[raw]
	n := alertEntities.LevelNotice{
		RuleEvaluationID: at.Unix(), MeasuringPointID: ruleEngineEntities.MotionPilotPointID, EventType: alertEntities.EventTypeLeak,
		EvaluatedAt: at, Status: alertEntities.StatusCalculated, IndicatorValue: &v,
		RawLevel: raw, ComputedLevel: computed, EpisodeLevel: computed, InEpisode: inEpisode, EpisodeEvent: event, HoursAbove: hoursAbove,
		Thresholds: alertEntities.Cuts{Amarillo: 0.16, Naranja: 0.25, Rojo: 0.35},
		Exit:       alertEntities.Cuts{Amarillo: 0.14, Naranja: 0.23, Rojo: 0.33}, ThresholdVersionID: 1,
	}
	if event == alertEntities.EpisodeCleared {
		n.BelowRun = 2
	}
	return n
}

// syntheticLeak son las horas de FUGA de §12.3, del 20 jul al 17 nov.
func syntheticLeak() []alertEntities.LevelNotice {
	episodes := []leakEpisode{
		{utc(7, 22, 6), utc(7, 22, 23), amarillo},
		{utc(7, 24, 0), utc(8, 3, 4), rojo},
		{utc(8, 22, 19), utc(8, 23, 5), amarillo},
		{utc(8, 24, 10), utc(8, 26, 0), rojo},    // alerta 4: el ruido termina el 25 ago 19:00 COL
		{utc(8, 27, 12), utc(8, 30, 5), naranja}, // y vuelve el 27 ago (LEAK_JOINED a las 08:00 COL)
		{utc(9, 19, 0), utc(9, 20, 1), amarillo},
		{utc(9, 26, 23), utc(9, 27, 6), amarillo},
		{utc(11, 13, 19), utc(11, 16, 14), amarillo},
	}
	var out []alertEntities.LevelNotice
	for at := utc(7, 20, 0); !at.After(utc(11, 17, 20)); at = at.Add(time.Hour) {
		var notice *alertEntities.LevelNotice
		for _, e := range episodes {
			hours := int(e.ended.Sub(e.started).Hours())
			k := int(at.Sub(e.started).Hours())
			switch {
			case at.Equal(e.started):
				n := leakHour(at, alertEntities.LevelAmarillo, alertEntities.LevelAmarillo, alertEntities.EpisodeNone, false, 0)
				notice = &n
			case at.Equal(e.ended):
				n := leakHour(at, alertEntities.LevelNormal, alertEntities.LevelNormal, alertEntities.EpisodeCleared, false, 0)
				notice = &n
			case at.After(e.started) && at.Before(e.ended):
				level := e.shape(k, hours)
				event := alertEntities.EpisodeNone
				if k == 1 {
					event = alertEntities.EpisodeOpened
				}
				n := leakHour(at, level, level, event, true, k+1)
				notice = &n
			}
		}
		if notice == nil {
			n := leakHour(at, alertEntities.LevelNormal, alertEntities.LevelNormal, alertEntities.EpisodeNone, false, 0)
			notice = &n
		}
		out = append(out, *notice)
	}
	return out
}

// realLeak lee el export de rule_evaluations (si existe) con el mismo mapeo que LevelSource.
func realLeak(t *testing.T, path string) []alertEntities.LevelNotice {
	t.Helper()
	rows := readCSVFile(t, path)
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	get := func(r []string, name string) string {
		if i, ok := col[name]; ok && i < len(r) {
			return r[i]
		}
		return ""
	}
	atoi := func(s string) int { v, _ := strconv.Atoi(s); return v }
	optInt := func(s string) *int {
		if s == "" {
			return nil
		}
		v := atoi(s)
		return &v
	}
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	// psql \copy escribe los booleanos como t / f.
	boolean := func(s string) bool { return s == "t" || s == "true" }
	out := make([]alertEntities.LevelNotice, 0, len(rows)-1)
	for _, r := range rows[1:] {
		at, err := time.Parse(time.RFC3339, get(r, "evaluated_at"))
		if err != nil {
			t.Fatalf("evaluated_at %q: %v", get(r, "evaluated_at"), err)
		}
		out = append(out, alertMappers.ToLevelNotice(alertModels.RuleEvaluationReadModel{
			ID: int64(atoi(get(r, "id"))), MeasuringPointID: int64(atoi(get(r, "measuring_point_id"))), EventType: "FUGA",
			EvaluatedAt: at, IndicatorValue: f(get(r, "indicator_value")), RawLevel: get(r, "raw_level"),
			ComputedLevel: get(r, "computed_level"), Level: get(r, "level"), InEpisode: boolean(get(r, "in_episode")),
			HoursAbove: atoi(get(r, "hours_above")), RunAmarillo: atoi(get(r, "run_amarillo")), RunNaranja: atoi(get(r, "run_naranja")),
			BelowRun: atoi(get(r, "below_run")), ThresholdVersionID: int64(atoi(get(r, "threshold_version_id"))),
			LowConfidence: boolean(get(r, "low_confidence")), EpisodeEvent: get(r, "episode_event"), Reasons: get(r, "reasons"),
			DPl: optFloatField(get(r, "d_pl")), DPlMin: optFloatField(get(r, "d_pl_min")),
			Amarillo: f(get(r, "amarillo")), Naranja: f(get(r, "naranja")), Rojo: f(get(r, "rojo")), ExitMargin: f(get(r, "exit_margin")),
			EscalateAmarilloAfterHours: optInt(get(r, "escalate_amarillo_after_hours")),
			EscalateNaranjaAfterHours:  optInt(get(r, "escalate_naranja_after_hours")),
			PersistenceMode:            get(r, "persistence_mode"), PersistenceMinHours: optInt(get(r, "persistence_min_hours")),
		}))
	}
	return out
}

// ---- fuentes en memoria para el replay ----

type sliceLevels []alertEntities.LevelNotice

func (s sliceLevels) ListPoints(context.Context) ([]int64, error) {
	return []int64{ruleEngineEntities.MotionPilotPointID}, nil
}
func (s sliceLevels) ListAfter(_ context.Context, _ int64, after, to time.Time, limit int) ([]alertEntities.LevelNotice, error) {
	i := sort.Search(len(s), func(k int) bool { return s[k].EvaluatedAt.After(after) })
	var out []alertEntities.LevelNotice
	for ; i < len(s) && len(out) < limit && (to.IsZero() || s[i].EvaluatedAt.Before(to)); i++ {
		out = append(out, s[i])
	}
	return out, nil
}

type sliceMotions []alertEntities.MotionNotice

func (s sliceMotions) ListPoints(context.Context) ([]int64, error) {
	return []int64{ruleEngineEntities.MotionPilotPointID}, nil
}
func (s sliceMotions) ListAfter(_ context.Context, _ int64, after, to time.Time, limit int) ([]alertEntities.MotionNotice, error) {
	i := sort.Search(len(s), func(k int) bool { return s[k].EvaluatedAt.After(after) })
	var out []alertEntities.MotionNotice
	for ; i < len(s) && len(out) < limit && (to.IsZero() || s[i].EvaluatedAt.Before(to)); i++ {
		out = append(out, s[i])
	}
	return out, nil
}

// ---- esperado (§12.3, en UTC: Colombia + 5 h) ----

type expectedAlert struct {
	event          alertEntities.EventType
	started, ended time.Time
	max            alertEntities.Level
	stableFrom     time.Time // cero = sin tramo
	stableTo       time.Time
	reactivations  int
}

var (
	fuga = alertEntities.EventTypeLeak
	mov  = alertEntities.EventTypeMovement
	at   = func(month time.Month, day, hour, min, sec int) time.Time {
		return time.Date(2026, month, day, hour, min, sec, 0, time.UTC)
	}
)

// §12.3 con FUGA y MOVIMIENTO SEPARADAS: la alerta 4 del spec (FUGA y
// MOVIMIENTO unidas) son ahora tres: la FUGA del 24–25 ago, la de MOVIMIENTO
// del 25–28 ago y una FUGA nueva el 27 ago (a más de 24 h de la anterior: no
// se reactiva). La FUGA del 27 ago recibe el TEMP_VARIES_AGAIN porque estaba
// vigente; la del 24–25 ago terminó antes del TEMP_STABILIZED.
var expected123 = []expectedAlert{
	{fuga, utc(7, 22, 6), utc(7, 22, 23), alertEntities.LevelAmarillo, time.Time{}, time.Time{}, 0},
	{fuga, utc(7, 24, 0), utc(8, 3, 4), alertEntities.LevelRojo, time.Time{}, time.Time{}, 0},
	{fuga, utc(8, 22, 19), utc(8, 23, 5), alertEntities.LevelAmarillo, time.Time{}, time.Time{}, 0},
	{fuga, utc(8, 24, 10), utc(8, 26, 0), alertEntities.LevelRojo, time.Time{}, time.Time{}, 0},
	{mov, utc(8, 25, 12), utc(8, 28, 21), alertEntities.LevelAmarillo, at(8, 25, 11, 15, 42), at(8, 27, 17, 45, 42), 0},
	{fuga, utc(8, 27, 12), utc(8, 30, 5), alertEntities.LevelNaranja, at(8, 25, 11, 15, 42), at(8, 27, 17, 45, 42), 0},
	{fuga, utc(9, 19, 0), utc(9, 20, 1), alertEntities.LevelAmarillo, time.Time{}, time.Time{}, 0},
	{fuga, utc(9, 26, 23), utc(9, 27, 6), alertEntities.LevelAmarillo, time.Time{}, time.Time{}, 0},
	{mov, utc(10, 12, 4), utc(10, 14, 10), alertEntities.LevelAmarillo, time.Time{}, time.Time{}, 1},
	{mov, utc(10, 20, 15), utc(10, 27, 1), alertEntities.LevelAmarillo, at(10, 20, 14, 15, 42), at(10, 25, 23, 30, 41), 0},
	{fuga, utc(11, 13, 19), utc(11, 16, 14), alertEntities.LevelAmarillo, time.Time{}, time.Time{}, 0},
}

// mensajes de movimiento y temperatura (y el fin) por alerta (código, hora UTC).
type timedCode struct {
	code string
	at   time.Time
}

var expectedMotionMessages = map[int][]timedCode{
	4: { // MOVIMIENTO de agosto
		{"MOV_OPEN", utc(8, 25, 12)}, {"MOV_DISPLACED", utc(8, 25, 14)}, {"TEMP_STABILIZED", utc(8, 26, 3)},
		{"MOV_RETURN", utc(8, 27, 19)}, {"TEMP_VARIES_AGAIN", utc(8, 27, 19)}, {"END_NORMAL", utc(8, 28, 21)},
	},
	5: { // FUGA del 27 ago: solo la temperatura que llegó estando vigente
		{"TEMP_VARIES_AGAIN", utc(8, 27, 19)}, {"END_NORMAL", utc(8, 30, 5)},
	},
	8: {
		{"MOV_OPEN", utc(10, 12, 6)}, {"END_NORMAL", utc(10, 13, 6)}, {"MOV_OPEN", utc(10, 13, 10)}, {"END_NORMAL", utc(10, 14, 10)},
	},
	9: {
		{"MOV_OPEN", utc(10, 21, 14)}, {"TEMP_STABILIZED", utc(10, 21, 19)}, {"MOV_RETURN", utc(10, 26, 1)},
		{"TEMP_VARIES_AGAIN", utc(10, 26, 1)}, {"END_NORMAL", utc(10, 27, 1)},
	},
}

// motionCodes son los códigos de movimiento (y el fin) que se comparan.
var motionCodes = []string{"MOV_OPEN", "MOV_DISPLACED", "MOV_RETURN", "TEMP_STABILIZED", "TEMP_VARIES_AGAIN",
	"TEMP_NOT_STABILIZED", "TEMP_STILL_STABLE", "END_NORMAL"}

func runBackfill123(t *testing.T, leak []alertEntities.LevelNotice) *alertMemory.AlertStore {
	t.Helper()
	log := logger.New("MotionAlertsTest", "Error")
	store := alertMemory.NewAlertStore()
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	processor := alertServices.NewProcessingService(store, alertMemory.NewPolicyStore(), noDevices{}, nil,
		alertPorts.ClockFunc(func() time.Time { return now }), log)
	replay := alertServices.NewReplayService(processor, sliceLevels(leak), store, alertServices.ReplayConfig{BatchSize: 500}, log).
		WithMotionSource(sliceMotions(referenceMotionNotices(t)))
	if _, err := replay.Backfill(context.Background(), alertServices.BackfillCommand{PointID: ruleEngineEntities.MotionPilotPointID}); err != nil {
		t.Fatal(err)
	}
	return store
}

func assertAlerts123(t *testing.T, store *alertMemory.AlertStore) {
	t.Helper()
	alerts := store.Alerts()
	sort.Slice(alerts, func(i, j int) bool { return alerts[i].StartedAt.Before(alerts[j].StartedAt) })
	if len(alerts) != len(expected123) {
		for _, a := range alerts {
			t.Logf("%s %s %v – %v %s", a.Code, a.EventType, a.StartedAt, a.EndedAt, a.MaxLevel)
		}
		t.Fatalf("alertas = %d, se esperaban %d", len(alerts), len(expected123))
	}
	for i, w := range expected123 {
		a := alerts[i]
		ok := a.EventType == w.event && a.StartedAt.Equal(w.started) &&
			a.EndedAt != nil && a.EndedAt.Equal(w.ended) && a.MaxLevel == w.max && a.ReactivationCount == w.reactivations &&
			a.State == alertEntities.AlertFinished && !a.Management.Classified()
		if w.stableFrom.IsZero() {
			ok = ok && a.TempStableFrom == nil
		} else {
			ok = ok && a.TempStableFrom != nil && a.TempStableFrom.Equal(w.stableFrom) && a.TempStableTo != nil && a.TempStableTo.Equal(w.stableTo)
		}
		if !ok {
			t.Errorf("alerta %d: %s %v – %v max %s reactivaciones %d temp %v – %v",
				i+1, a.EventType, a.StartedAt, a.EndedAt, a.MaxLevel, a.ReactivationCount, a.TempStableFrom, a.TempStableTo)
		}
	}
	for i, want := range expectedMotionMessages {
		var got []timedCode
		for _, m := range store.Messages() {
			if m.AlertID == alerts[i].ID && slices.Contains(motionCodes, m.Code) {
				got = append(got, timedCode{m.Code, m.EvaluatedAt})
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("alerta %d: mensajes\n got  %v\n want %v", i+1, got, want)
		}
	}
	// Una alerta de FUGA nunca lleva mensajes del sensor, y nunca hay dos
	// activas del mismo evento a la vez (sí una de cada uno).
	for _, a := range alerts {
		for _, m := range store.Messages() {
			if m.AlertID == a.ID && a.EventType == fuga && strings.HasPrefix(m.Code, "MOV_") {
				t.Errorf("%s (FUGA) tiene %s el %v", a.Code, m.Code, m.EvaluatedAt)
			}
		}
	}
	for i := 1; i < len(alerts); i++ {
		for j := 0; j < i; j++ {
			if alerts[i].EventType == alerts[j].EventType && alerts[i].StartedAt.Before(*alerts[j].EndedAt) {
				t.Errorf("%s y %s (%s) se solapan", alerts[j].Code, alerts[i].Code, alerts[i].EventType)
			}
		}
	}
}

func TestBackfill_ReproducesTheAlertsOf12_3(t *testing.T) {
	assertAlerts123(t, runBackfill123(t, syntheticLeak()))
}

func TestBackfill_ReproducesTheAlertsOf12_3WithRealLeakHistory(t *testing.T) {
	path := motionTestdataDir + "rule_evaluations_11556.csv"
	if _, err := os.Stat(path); err != nil {
		t.Skip("falta " + path + " (export de rule_evaluations del MP-11556; ver RULE_ENGINE_README)")
	}
	assertAlerts123(t, runBackfill123(t, realLeak(t, path)))
}
