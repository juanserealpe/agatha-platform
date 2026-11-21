package services

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

// Prueba dorada AM (CA-3, REQ_MOVIMIENTO_UMBRALES_V2): la salida del Rule
// Engine (motion_v2_expected*.csv, la misma que reproduce MotionAssess)
// pasada por DecideMotion v8 sin FUGA da las alertas y los textos exactos de
// alert_messages_v2_expected*.json (messages_v2_ref.py). Los códigos v7 sin
// cambios (TEMP_*) se comparan solo por código y hora. Los anexos viven con
// los del Rule Engine.
const motionV2Testdata = "../../../rule_engine/domain/services/testdata/motion_v2/"

var amV2Cases = []struct{ name, csv, json string }{
	{"real", "motion_v2_expected.csv", "alert_messages_v2_expected.json"},
	{"creep", "motion_v2_expected_creep.csv", "alert_messages_v2_expected_creep.json"},
	{"escalones", "motion_v2_expected_escalones.csv", "alert_messages_v2_expected_escalones.json"},
	{"rapido_grande", "motion_v2_expected_rapido_grande.csv", "alert_messages_v2_expected_rapido_grande.json"},
	{"rapido_rojo", "motion_v2_expected_rapido_rojo.csv", "alert_messages_v2_expected_rapido_rojo.json"},
}

type expectedAlert struct {
	Alert         string `json:"alert"`
	StartedAt     string `json:"started_at"`
	EndedAt       string `json:"ended_at"`
	Reactivations int    `json:"reactivations"`
	MaxLevel      string `json:"max_level"`
	MaxCalc       string `json:"max_calc"`
	Messages      []struct {
		EvaluatedAt string `json:"evaluated_at"`
		Code        string `json:"code"`
		Kind        string `json:"kind"`
		Title       string `json:"title"`
		Body        string `json:"body"`
	} `json:"messages"`
}

func parseRefTime(t *testing.T, s string) time.Time {
	t.Helper()
	for _, layout := range []string{"2006-01-02 15:04:05-07:00", time.RFC3339} {
		if v, err := time.Parse(layout, s); err == nil {
			return v.UTC()
		}
	}
	t.Fatalf("hora %q", s)
	return time.Time{}
}

func optTime(t *testing.T, s string) *time.Time {
	if s == "" {
		return nil
	}
	v := parseRefTime(t, s)
	return &v
}

func optNum(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// loadMotionNotices traduce cada fila de motion_v2_expected*.csv a la
// MotionNotice que le llegaría a Alert Management (evento v2).
func loadMotionNotices(t *testing.T, name string) []entities.MotionNotice {
	t.Helper()
	f, err := os.Open(motionV2Testdata + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	head := recs[0]
	var out []entities.MotionNotice
	for _, rec := range recs[1:] {
		r := map[string]string{}
		for i, h := range head {
			r[h] = rec[i]
		}
		n := entities.MotionNotice{
			MeasuringPointID: pointID, EvaluatedAt: parseRefTime(t, r["evaluated_at"]), Status: r["status"],
			MotionState: r["motion_state"], TempState: r["temp_state"], TempMode: "MOVIMIENTO_Y_TEMPERATURA",
			ComputedLevel: entities.Level(r["computed_level"]), InEpisode: r["in_episode"] == "True",
			EpisodeEvent: entities.EpisodeEvent(r["episode_event"]), TableLevel: entities.Level(r["table_level"]),
			EpisodeKind: r["episode_kind"], ShiftStartedAt: optTime(t, r["shift_started_at"]),
			TempStableFrom: optTime(t, r["temp_stable_from"]), TempStableTo: optTime(t, r["temp_stable_to"]),
			AlertsEnabled: true, SchemaVersion: 2,
		}
		if p, ref := optNum(r["pitch_now"]), optNum(r["reference_pitch"]); p != nil && ref != nil {
			v := *p - *ref
			n.ShiftPitch = &v
		}
		if p, ref := optNum(r["roll_now"]), optNum(r["reference_roll"]); p != nil && ref != nil {
			v := *p - *ref
			n.ShiftRoll = &v
		}
		var reasons []struct {
			Code string         `json:"code"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal([]byte(r["reasons"]), &reasons); err != nil {
			t.Fatalf("%s reasons: %v", r["evaluated_at"], err)
		}
		for _, x := range reasons {
			n.Reasons = append(n.Reasons, entities.ParseMotionReason(x.Code, x.Data))
		}
		if r["history"] != "" {
			if err := json.Unmarshal([]byte(r["history"]), &n.History); err != nil {
				t.Fatalf("%s history: %v", r["evaluated_at"], err)
			}
		}
		if r["rules_hit"] != "" {
			_ = json.Unmarshal([]byte(r["rules_hit"]), &n.RulesHit)
		}
		out = append(out, n)
	}
	return out
}

// runMotionAlerts pasa las horas por DecideMotion v8 (sin FUGA) y devuelve
// el mundo con las alertas y los mensajes.
func runMotionAlerts(t *testing.T, notices []entities.MotionNotice) *cworld {
	w := newCWorld(t)
	w.policy.EventType = entities.EventTypeMovement
	w.mov(notices...)
	return w
}

func (w *cworld) movementAlerts() []*entities.Alert {
	var out []*entities.Alert
	for _, a := range w.alerts {
		if a.EventType == entities.EventTypeMovement {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// v7Unchanged: códigos que solo se comparan por código y hora (CA-3).
func v7Unchanged(code string) bool { return strings.HasPrefix(code, "TEMP_") }

func TestDecideMotionV8_Golden(t *testing.T) {
	for _, tc := range amV2Cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(motionV2Testdata + tc.json)
			if err != nil {
				t.Fatal(err)
			}
			var want []expectedAlert
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			w := runMotionAlerts(t, loadMotionNotices(t, tc.csv))
			got := w.movementAlerts()
			if len(got) != len(want) {
				t.Fatalf("alertas = %d, se esperaban %d", len(got), len(want))
			}
			var diffs []string
			for i, wa := range want {
				ga := got[i]
				ended := "None"
				if ga.EndedAt != nil {
					ended = ga.EndedAt.Format("2006-01-02 15:04:05-07:00")
				}
				wantEnded := wa.EndedAt
				if wantEnded != "None" {
					wantEnded = parseRefTime(t, wa.EndedAt).Format("2006-01-02 15:04:05-07:00")
				}
				switch {
				case !ga.StartedAt.Equal(parseRefTime(t, wa.StartedAt)):
					diffs = append(diffs, fmt.Sprintf("%s started_at %v, se esperaba %s", wa.Alert, ga.StartedAt, wa.StartedAt))
				case ended != wantEnded:
					diffs = append(diffs, fmt.Sprintf("%s ended_at %s, se esperaba %s", wa.Alert, ended, wa.EndedAt))
				case ga.ReactivationCount != wa.Reactivations:
					diffs = append(diffs, fmt.Sprintf("%s reactivaciones %d, se esperaban %d", wa.Alert, ga.ReactivationCount, wa.Reactivations))
				case string(ga.MaxLevel) != wa.MaxLevel || string(ga.MotionTableLevel) != wa.MaxCalc:
					diffs = append(diffs, fmt.Sprintf("%s nivel %s/%s, se esperaba %s/%s", wa.Alert, ga.MaxLevel, ga.MotionTableLevel, wa.MaxLevel, wa.MaxCalc))
				}
				var gm []entities.Message
				for _, m := range w.messages {
					if m.AlertID == ga.ID {
						gm = append(gm, m)
					}
				}
				if len(gm) != len(wa.Messages) {
					var codes []string
					for _, m := range gm {
						codes = append(codes, m.EvaluatedAt.Format(time.RFC3339)+" "+m.Code)
					}
					diffs = append(diffs, fmt.Sprintf("%s mensajes = %d, se esperaban %d: %v", wa.Alert, len(gm), len(wa.Messages), codes))
					continue
				}
				for k, wm := range wa.Messages {
					g := gm[k]
					at := parseRefTime(t, wm.EvaluatedAt)
					if !g.EvaluatedAt.Equal(at) || g.Code != wm.Code {
						diffs = append(diffs, fmt.Sprintf("%s #%d: %v %s, se esperaba %s %s", wa.Alert, k, g.EvaluatedAt, g.Code, wm.EvaluatedAt, wm.Code))
						continue
					}
					if v7Unchanged(wm.Code) {
						continue
					}
					if string(g.Kind) != wm.Kind || g.Title != wm.Title || g.Body != wm.Body {
						diffs = append(diffs, fmt.Sprintf("%s %s %s:\n  got  [%s] %s — %s\n  want [%s] %s — %s",
							wa.Alert, wm.EvaluatedAt, wm.Code, g.Kind, g.Title, g.Body, wm.Kind, wm.Title, wm.Body))
					}
					if g.CatalogVersion != messages.MessageCatalogVersion {
						diffs = append(diffs, fmt.Sprintf("%s %s: catalog_version %d", wa.Alert, wm.Code, g.CatalogVersion))
					}
				}
			}
			for _, d := range diffs {
				t.Error(d)
			}
		})
	}
}

// §10.5: todo texto de movimiento renderizado cita al menos un número en
// grados y ninguno deja una variable sin reemplazar. Excepción (manda la
// referencia, §10.2): MOV_DAILY sin datos y con datos parciales sin el total
// no tienen grados que citar (creep, 07/11 08:00 COT).
func TestDecideMotionV8_TextsCiteDegrees(t *testing.T) {
	for _, tc := range amV2Cases {
		w := runMotionAlerts(t, loadMotionNotices(t, tc.csv))
		for _, m := range w.messages {
			if strings.Contains(m.Title+m.Body, "{") {
				t.Errorf("%s %s %v: variable sin reemplazar: %s", tc.name, m.Code, m.EvaluatedAt, m.Body)
			}
			if !strings.HasPrefix(m.Code, "MOV_") && m.Code != "END_NORMAL" {
				continue
			}
			noData := m.Body == "Sin datos del sensor en las últimas 24 h." ||
				(m.Params["n_horas"] != "" && m.Params["total"] == "")
			if m.Code == "MOV_DAILY" && noData {
				continue
			}
			if !strings.Contains(m.Body, "°") {
				t.Errorf("%s %s %v sin grados: %s", tc.name, m.Code, m.EvaluatedAt, m.Body)
			}
			if m.Params["datos"] == "" {
				t.Errorf("%s %s %v sin params[datos]", tc.name, m.Code, m.EvaluatedAt)
			}
		}
	}
}
