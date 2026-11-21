package services

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// Pruebas doradas motion-v2 (CA-1 y CA-2): MotionAssess hora a hora sobre las
// entradas de testdata/motion_v2 con la tabla de §5.3 debe reproducir
// motion_v2_expected*.csv (salida de motion_v2_ref.py).

var v2Cases = []struct{ name, input, expected string }{
	{"real", "motion_input.csv", "motion_v2_expected.csv"},
	{"creep", "motion_input_creep.csv", "motion_v2_expected_creep.csv"},
	{"escalones", "motion_input_escalones.csv", "motion_v2_expected_escalones.csv"},
	{"rapido_grande", "motion_input_rapido_grande.csv", "motion_v2_expected_rapido_grande.csv"},
	{"rapido_rojo", "motion_input_rapido_rojo.csv", "motion_v2_expected_rapido_rojo.csv"},
}

// v2ExactCols son las columnas nuevas que se comparan como texto.
var v2ExactCols = []string{"table_level_hour", "table_level", "episode_kind", "rest_at", "ep_start_at", "ep_has_long",
	"last_pattern_at", "last_fast_at"}

var v2NumCols = []string{"ep_ref_pitch", "ep_ref_roll"}

func gotRowV2(e entities.MotionEvaluation) (map[string]string, map[string]any) {
	r, _ := gotRow(e)
	r["table_level"] = string(e.TableLevel)
	r["episode_kind"] = string(e.EpisodeKind)
	s := e.State
	r["rest_at"], r["last_pattern_at"], r["last_fast_at"] = tstr(s.RestAt), tstr(s.LastPatternAt), tstr(s.LastFastAt)
	r["ep_start_at"], r["ep_ref_pitch"], r["ep_ref_roll"] = "", "", ""
	r["ep_has_long"] = bstr(false)
	if s.InEpisode {
		r["ep_start_at"], r["ep_ref_pitch"], r["ep_ref_roll"] = tstr(s.EpStartAt), fstr(s.EpRefPitch), fstr(s.EpRefRoll)
		r["ep_has_long"] = bstr(s.EpHasLong)
	}
	js := map[string]any{}
	hold := map[string]int{}
	for k, v := range s.LongHold {
		hold[k] = v
	}
	js["long_hold"] = hold
	if e.Status == entities.MotionStatusCalculated {
		r["table_level_hour"] = string(e.TableLevelHour)
		hits := e.RulesHit
		if hits == nil {
			hits = []entities.MotionRuleHit{}
		}
		js["rules_hit"] = hits
	} else {
		r["table_level_hour"] = ""
	}
	reasons := make([]map[string]any, 0, len(e.Reasons))
	for _, rs := range e.Reasons {
		reasons = append(reasons, map[string]any{"code": string(rs.Code), "data": rs.Data()})
	}
	js["reasons"] = reasons
	if e.History != nil {
		js["history"] = e.History
	}
	// Ida y vuelta por JSON para comparar con el esperado.
	out := map[string]any{}
	for k, v := range js {
		b, _ := json.Marshal(v)
		var x any
		_ = json.Unmarshal(b, &x)
		out[k] = x
	}
	return r, out
}

// sameSet compara dos arreglos JSON sin importar el orden.
func sameSet(a, b any) bool {
	x, ok1 := a.([]any)
	y, ok2 := b.([]any)
	if !ok1 || !ok2 || len(x) != len(y) {
		return ok1 == ok2 && len(x) == len(y) && len(x) == 0
	}
	used := make([]bool, len(y))
	for _, v := range x {
		found := false
		for j, w := range y {
			if !used[j] && anyEqual(v, w) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func jsonCell(t *testing.T, at, col, s string) any {
	t.Helper()
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("%s %s: %v", at, col, err)
	}
	return v
}

func TestMotionAssessV2_Golden(t *testing.T) {
	for _, tc := range v2Cases {
		t.Run(tc.name, func(t *testing.T) {
			records := loadMotionCSV(t, "testdata/motion_v2/"+tc.input)
			want := readCSV(t, "testdata/motion_v2/"+tc.expected)
			got := runMotion(records, entities.DefaultMotionParams())
			if len(got) != len(want) {
				t.Fatalf("horas = %d, se esperaban %d", len(got), len(want))
			}
			var diffs []string
			for i, w := range want {
				at := w["evaluated_at"]
				g, gj := gotRowV2(got[i])
				for _, c := range append(append([]string{}, exactCols...), v2ExactCols...) {
					if g[c] != w[c] {
						diffs = append(diffs, fmt.Sprintf("%s %s: got %q want %q", at, c, g[c], w[c]))
					}
				}
				for _, c := range append(append([]string{}, numCols...), v2NumCols...) {
					if !numEqual(g[c], w[c]) {
						diffs = append(diffs, fmt.Sprintf("%s %s: got %q want %q", at, c, g[c], w[c]))
					}
				}
				if w["status"] == string(entities.MotionStatusCalculated) {
					if wh := jsonCell(t, at, "rules_hit", w["rules_hit"]); !sameSet(gj["rules_hit"], wh) {
						gb, _ := json.Marshal(gj["rules_hit"])
						diffs = append(diffs, fmt.Sprintf("%s rules_hit:\n  got  %s\n  want %s", at, gb, w["rules_hit"]))
					}
				}
				if wr := jsonCell(t, at, "reasons", w["reasons"]); !sameSet(gj["reasons"], wr) {
					gb, _ := json.Marshal(gj["reasons"])
					diffs = append(diffs, fmt.Sprintf("%s reasons:\n  got  %s\n  want %s", at, gb, w["reasons"]))
				}
				if wl := jsonCell(t, at, "long_hold", w["long_hold"]); !anyEqual(normHold(gj["long_hold"]), normHold(wl)) {
					gb, _ := json.Marshal(gj["long_hold"])
					diffs = append(diffs, fmt.Sprintf("%s long_hold: got %s want %s", at, gb, w["long_hold"]))
				}
				wh := jsonCell(t, at, "history", w["history"])
				if (wh == nil) != (gj["history"] == nil) || (wh != nil && !anyEqual(gj["history"], wh)) {
					gb, _ := json.Marshal(gj["history"])
					diffs = append(diffs, fmt.Sprintf("%s history:\n  got  %s\n  want %s", at, gb, w["history"]))
				}
			}
			if len(diffs) > 0 {
				max := len(diffs)
				if max > 30 {
					max = 30
				}
				for _, d := range diffs[:max] {
					t.Error(d)
				}
				t.Fatalf("%d diferencias con %s", len(diffs), tc.expected)
			}
		})
	}
}

// normHold trata nil y {} igual.
func normHold(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

// TestMotionAssessV2_Episodes resume los episodios de cada caso (§11) para
// que una regresión se lea rápido.
func TestMotionAssessV2_Episodes(t *testing.T) {
	want := map[string][]string{
		"real": {"2026-08-25T12:00:00Z SALTO AMARILLO", "2026-10-12T06:00:00Z RAPIDO AMARILLO",
			"2026-10-13T10:00:00Z RAPIDO AMARILLO", "2026-10-21T14:00:00Z HUECO AMARILLO"},
	}
	for _, tc := range v2Cases[:1] {
		records := loadMotionCSV(t, "testdata/motion_v2/"+tc.input)
		var got []string
		for _, e := range runMotion(records, entities.DefaultMotionParams()) {
			if e.EpisodeEvent == entities.EpisodeEventOpened {
				got = append(got, tstr(&e.EvaluatedAt)+" "+string(e.EpisodeKind)+" "+string(e.TableLevel))
			}
		}
		sort.Strings(got)
		if fmt.Sprint(got) != fmt.Sprint(want[tc.name]) {
			t.Errorf("%s: episodios %v, se esperaban %v", tc.name, got, want[tc.name])
		}
	}
}

// CA-9: lo acumulado no se evalúa en ventanas con patrón conjunto o cambio
// rápido. CA-10: MOV_DAILY sale también en horas NO_CALCULABLE y nunca en la
// hora del CLEARED.
func TestMotionAssessV2_CumulativeAndDailyRules(t *testing.T) {
	runCase := func(name string) []entities.MotionEvaluation {
		for _, tc := range v2Cases {
			if tc.name == name {
				return runMotion(loadMotionCSV(t, "testdata/motion_v2/"+tc.input), entities.DefaultMotionParams())
			}
		}
		t.Fatalf("caso %s", name)
		return nil
	}
	for _, name := range []string{"rapido_grande", "rapido_rojo"} {
		for _, e := range runCase(name) {
			if e.HasReason(entities.ReasonMovCumulative) {
				t.Errorf("%s %v: MOV_CUMULATIVE en un cambio rápido", name, e.EvaluatedAt)
			}
		}
	}
	noDataAt := time.Date(2026, 11, 8, 13, 0, 0, 0, time.UTC) // 08/11 08:00 COT
	found := false
	for _, e := range runCase("creep") {
		if e.HasReason(entities.ReasonMovDaily) && e.EpisodeEvent == entities.EpisodeEventCleared {
			t.Errorf("%v: MOV_DAILY en la hora del CLEARED", e.EvaluatedAt)
		}
		if e.EvaluatedAt.Equal(noDataAt) {
			r, ok := reasonOf(e, entities.ReasonMovDaily)
			found = ok && r.NoData && e.Status == entities.MotionStatusNoCalculable
		}
	}
	if !found {
		t.Errorf("creep %v: se esperaba MOV_DAILY sin datos en una hora NO_CALCULABLE", noDataAt)
	}
}

// §8 (P2): observed con la tabla de §5.3 sobre los 90 días reales del
// MP-11556, fuera de episodios, da los máximos de §2.
func TestMotionObserved_ReproducesSection2(t *testing.T) {
	rows := runMotion(loadMotionCSV(t, "testdata/motion_v2/motion_input.csv"), entities.DefaultMotionParams())
	end := rows[len(rows)-1].EvaluatedAt
	got := MotionObserved(rows, entities.DefaultMotionRules(), end.Add(-90*24*time.Hour), end)
	want := map[string]float64{
		"1:pitch": 0.01, "1:roll": 0.025, "3:pitch": 0.01, "3:roll": 0.03, "72:pitch": 0.025, "72:roll": 0.05,
		"168:pitch": 0.04, "168:roll": 0.04, "720:pitch": 0.07, "720:roll": 0.095,
	}
	if len(got) != len(want) {
		t.Fatalf("filas = %d, se esperaban %d", len(got), len(want))
	}
	for _, r := range got {
		key := fmt.Sprintf("%d:%s", r.WindowHours, r.Axis)
		if r.Max == nil || math.Abs(*r.Max-want[key]) > 1e-9 || r.P95 == nil || *r.P95 > *r.P99 || *r.P99 > *r.Max || r.Samples == 0 {
			t.Errorf("%s: %+v, máximo esperado %v", key, r, want[key])
		}
	}
}
