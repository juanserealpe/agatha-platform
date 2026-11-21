package services

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// Prueba dorada v1 (§12.1 de v1) vista desde motion-v2 (CA-1, §11.1): las
// columnas v1 de motion-v2 coinciden con motion_expected.csv en todas las
// horas. Los motivos NO se comparan aquí: motion-v2 agrega MOV_DAILY y
// MOV_NEW_POSITION y cambia los datos de MOV_ABRUPT y MOV_STILL; los compara
// TestMotionAssessV2_Golden contra motion_v2_expected.csv.

func loadMotionInput(t *testing.T) []entities.RawRecord {
	t.Helper()
	return loadMotionCSV(t, "testdata/motion/motion_input.csv")
}

func loadMotionCSV(t *testing.T, path string) []entities.RawRecord {
	t.Helper()
	rows := readCSV(t, path)
	out := make([]entities.RawRecord, 0, len(rows))
	for _, r := range rows {
		ts, err := time.Parse(time.RFC3339, r["timestamp"])
		if err != nil {
			t.Fatalf("timestamp %q: %v", r["timestamp"], err)
		}
		out = append(out, entities.RawRecord{
			Timestamp: ts, Pitch: optFloat(t, r["pitch"]), Roll: optFloat(t, r["roll"]),
			Deriva: optFloat(t, r["deriva"]), Temp: optFloat(t, r["temp"]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}

func readCSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("abrir %s: %v", path, err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("leer %s: %v", path, err)
	}
	head := recs[0]
	out := make([]map[string]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		m := make(map[string]string, len(head))
		for i, h := range head {
			m[h] = rec[i]
		}
		out = append(out, m)
	}
	return out
}

func optFloat(t *testing.T, s string) *float64 {
	t.Helper()
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("número %q: %v", s, err)
	}
	return &v
}

// runMotion evalúa todas las horas, de ceil(primer registro con pitch y roll)
// a floor(último), encadenando el estado y los episodios cerrados como lo
// haría el scheduler.
func runMotion(records []entities.RawRecord, p entities.MotionParams) []entities.MotionEvaluation {
	var first, last time.Time
	for _, r := range records {
		if r.Pitch == nil || r.Roll == nil {
			continue
		}
		if first.IsZero() {
			first = r.Timestamp
		}
		last = r.Timestamp
	}
	t0 := first.Truncate(time.Hour)
	if !t0.Equal(first) {
		t0 = t0.Add(time.Hour)
	}
	t1 := last.Truncate(time.Hour)
	state := entities.InitialMotionState(p)
	var out []entities.MotionEvaluation
	var past []entities.MotionEpisode
	for t := t0; !t.After(t1); t = t.Add(time.Hour) {
		from := t.Add(-MotionWindowHours * time.Hour)
		i := sort.Search(len(records), func(k int) bool { return !records[k].Timestamp.Before(from) })
		j := sort.Search(len(records), func(k int) bool { return !records[k].Timestamp.Before(t) })
		ev := MotionAssess(records[i:j], state, p, t, past)
		state = ev.State
		if ep, ok := entities.EpisodeFromCleared(ev); ok {
			past = append(past, ep)
		}
		out = append(out, ev)
	}
	return out
}

func fstr(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}

func tstr(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

func bstr(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// gotRow arma la fila de la evaluación con las columnas de motion_expected.csv.
func gotRow(e entities.MotionEvaluation) (map[string]string, []map[string]any) {
	r := map[string]string{
		"evaluated_at": tstr(&e.EvaluatedAt),
		"n_records":    strconv.Itoa(e.NRecords),
		"coverage":     strconv.FormatFloat(e.Coverage, 'g', -1, 64),
		"status":       string(e.Status),
		"motion_state": string(e.MotionState),
		"temp_state":   string(e.State.TempState),
		"in_episode":   bstr(e.State.InEpisode),
		"episode_event": func() string {
			if e.Status == entities.MotionStatusNoCalculable {
				return ""
			}
			return string(e.EpisodeEvent)
		}(),
		"computed_level":   string(e.ComputedLevel),
		"reference_pitch":  fstr(e.ReferencePitch()),
		"reference_roll":   fstr(e.ReferenceRoll()),
		"shift_started_at": tstr(e.ShiftStartedAt()),
		"temp_stable_from": tstr(e.TempStableFrom()),
		"temp_stable_to":   tstr(e.TempStableTo()),
	}
	if e.Status == entities.MotionStatusCalculated {
		r["pitch_now"], r["roll_now"], r["temp_now"] = fstr(e.PitchNow), fstr(e.RollNow), fstr(e.TempNow)
		r["dp_1h"], r["dr_1h"], r["dp_6h"], r["dr_6h"] = fstr(e.DP1h), fstr(e.DR1h), fstr(e.DP6h), fstr(e.DR6h)
		r["shift_records_2h"], r["shift_sign"] = strconv.Itoa(e.ShiftRecords2h), strconv.Itoa(e.ShiftSignNow)
		r["deriva_max_1h"], r["jump"] = fstr(e.DerivaMax1h), bstr(e.Jump)
		r["dp_24h"], r["dr_24h"], r["trend_flag"] = fstr(e.DP24h), fstr(e.DR24h), bstr(e.TrendFlag)
		r["temp_rate_max_6h"], r["temp_rate_last"] = fstr(e.TempRateMax6h), fstr(e.TempRate)
		r["temp_flat"], r["temp_flow"], r["abrupt"] = bstr(e.TempFlat), bstr(e.TempFlow), bstr(e.Abrupt)
		r["slow_drift_flag"], r["still"] = bstr(e.SlowDriftFlag), bstr(e.Still)
		r["gap_from"], r["gap_to"], r["dp_gap"], r["dr_gap"] = tstr(e.EvalGapFrom), tstr(e.EvalGapTo), fstr(e.DPGap), fstr(e.DRGap)
	} else {
		r["episode_event"] = "NONE"
		for _, c := range []string{"pitch_now", "roll_now", "temp_now", "dp_1h", "dr_1h", "dp_6h", "dr_6h",
			"shift_records_2h", "shift_sign", "deriva_max_1h", "jump", "dp_24h", "dr_24h", "trend_flag",
			"temp_rate_max_6h", "temp_rate_last", "temp_flat", "temp_flow", "abrupt", "slow_drift_flag", "still",
			"gap_from", "gap_to", "dp_gap", "dr_gap"} {
			r[c] = ""
		}
	}
	reasons := make([]map[string]any, 0, len(e.Reasons))
	for _, rs := range e.Reasons {
		reasons = append(reasons, map[string]any{"code": string(rs.Code), "data": rs.Data()})
	}
	// Ida y vuelta por JSON para que los números queden como en el esperado.
	b, _ := json.Marshal(reasons)
	var norm []map[string]any
	_ = json.Unmarshal(b, &norm)
	return r, norm
}

var exactCols = []string{"evaluated_at", "status", "motion_state", "temp_state", "in_episode", "episode_event",
	"computed_level", "shift_started_at", "temp_stable_from", "temp_stable_to", "jump", "abrupt", "still",
	"temp_flat", "temp_flow", "trend_flag", "slow_drift_flag", "gap_from", "gap_to"}

var numCols = []string{"n_records", "coverage", "pitch_now", "roll_now", "temp_now", "dp_1h", "dr_1h", "dp_6h",
	"dr_6h", "shift_records_2h", "shift_sign", "deriva_max_1h", "dp_24h", "dr_24h", "temp_rate_max_6h",
	"temp_rate_last", "reference_pitch", "reference_roll", "dp_gap", "dr_gap"}

func numEqual(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	x, err1 := strconv.ParseFloat(a, 64)
	y, err2 := strconv.ParseFloat(b, 64)
	return err1 == nil && err2 == nil && math.Abs(x-y) <= 1e-6
}

func anyEqual(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		return ok && math.Abs(x-y) <= 1e-6
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok || !anyEqual(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !anyEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

func TestMotionAssess_GoldenMP11556_V1Columns(t *testing.T) {
	records := loadMotionInput(t)
	want := readCSV(t, "testdata/motion/motion_expected.csv")
	got := runMotion(records, entities.DefaultMotionParams())

	if len(got) != len(want) {
		t.Fatalf("horas = %d, se esperaban %d", len(got), len(want))
	}
	var diffs []string
	for i, w := range want {
		g, _ := gotRow(got[i])
		for _, c := range exactCols {
			if g[c] != w[c] {
				diffs = append(diffs, fmt.Sprintf("%s %s: got %q want %q", w["evaluated_at"], c, g[c], w[c]))
			}
		}
		for _, c := range numCols {
			if !numEqual(g[c], w[c]) {
				diffs = append(diffs, fmt.Sprintf("%s %s: got %q want %q", w["evaluated_at"], c, g[c], w[c]))
			}
		}
	}
	if len(diffs) > 0 {
		max := len(diffs)
		if max > 40 {
			max = 40
		}
		for _, d := range diffs[:max] {
			t.Error(d)
		}
		t.Fatalf("%d diferencias con motion_expected.csv", len(diffs))
	}
}
