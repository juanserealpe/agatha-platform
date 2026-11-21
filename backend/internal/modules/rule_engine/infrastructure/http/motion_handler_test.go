package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	ruleEngineHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type emptyRawSource struct{}

func (emptyRawSource) ListRecords(context.Context, int64, time.Time, time.Time) ([]entities.RawRecord, error) {
	return nil, nil
}
func (emptyRawSource) FirstMeasurementAt(context.Context, int64) (*time.Time, error) { return nil, nil }
func (emptyRawSource) ListPoints(context.Context) ([]int64, error)                   { return nil, nil }

func newMotionServer() *http.ServeMux {
	log := logger.New("RuleEngineHTTPTest", "Error")
	clock := ports.ClockFunc(func() time.Time { return now })
	repo := memory.NewMotionThresholdRepository(entities.MotionThresholdVersion{
		MeasuringPointID: pointID, Version: 1, Params: entities.DefaultMotionParams(), ValidFrom: day1,
		Reason: entities.SeedReason, CreatedBy: entities.SystemCreatedBy, CreatedAt: day1,
	})
	mux := http.NewServeMux()
	ruleEngineHTTP.NewMotionHandler(
		services.NewMotionThresholdService(repo, clock, log),
		services.NewMotionSimulationService(repo, emptyRawSource{}, clock, log),
		log,
	).RegisterRoutes(mux)
	return mux
}

func call(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestMotionHandler_GetCurrentReturnsTheParams(t *testing.T) {
	rec := call(newMotionServer(), http.MethodGet, "/api/rule-engine/points/11556/motion-thresholds", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var v ruleEngineHTTP.MotionVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.Version != 1 || v.Params["jump_deg"] != 0.03 || v.Params["temp_mode"] != "MOVIMIENTO_Y_TEMPERATURA" ||
		v.Params["alerts_enabled"] != true || v.Params["send_naranja"] != "OBSERVATION" || v.Params["closure_max_deg"] != 0.2 {
		t.Fatalf("respuesta %+v", v)
	}
	// Todos los parámetros menos los retirados, más la tabla.
	if _, ok := v.Params["abrupt_deg"]; ok || len(v.Params) != len(entities.MotionParamFields())-2+1 {
		t.Fatalf("parámetros %v", v.Params)
	}
	rules, _ := v.Params["rules"].([]any)
	if len(rules) != 5 || rules[0].(map[string]any)["rojo_deg"] != 0.2 || rules[0].(map[string]any)["naranja_deg"] != nil {
		t.Fatalf("rules %v", v.Params["rules"])
	}
}

func TestMotionHandler_CreateAndHistory(t *testing.T) {
	mux := newMotionServer()
	rec := call(mux, http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
		`{"patch": {"rules": [{"window_hours": 3, "axis": "PITCH", "amarillo_deg": 0.05, "naranja_deg": null, "rojo_deg": 0.3}],
		  "send_rojo": "ACTIVE", "temp_mode": "SOLO_MOVIMIENTO"}, "reason": "calibración", "created_by": "ana"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var v ruleEngineHTTP.MotionVersionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	rules, _ := v.Params["rules"].([]any)
	if v.Version != 2 || v.Params["send_rojo"] != "ACTIVE" || v.Params["temp_mode"] != "SOLO_MOVIMIENTO" || v.PreviousVersionID == nil ||
		len(rules) != 1 || len(v.Warnings) != 1 || v.Warnings[0] != entities.MotionWarnNoLongRule {
		t.Fatalf("versión %+v", v)
	}

	rec = call(mux, http.MethodGet, "/api/rule-engine/points/11556/motion-thresholds/history", "")
	var history []ruleEngineHTTP.MotionVersionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &history)
	if rec.Code != http.StatusOK || len(history) != 2 || history[0].ValidTo == nil ||
		len(history[0].Params["rules"].([]any)) != 5 || len(history[1].Params["rules"].([]any)) != 1 {
		t.Fatalf("historial %d: %s", rec.Code, rec.Body)
	}

	rec = call(mux, http.MethodGet, "/api/rule-engine/points/11556/motion-thresholds/timeline?from=2026-07-01T00:00:00Z&to=2027-01-01T00:00:00Z", "")
	var segs []ruleEngineHTTP.MotionTimelineSegmentResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &segs)
	if rec.Code != http.StatusOK || len(segs) != 2 || !segs[0].From.Equal(day1) || !segs[0].To.Equal(segs[1].From) ||
		!segs[1].To.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("timeline %d: %s", rec.Code, rec.Body)
	}
}

func TestMotionHandler_RejectsInvalidRequests(t *testing.T) {
	mux := newMotionServer()
	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{"punto inválido", http.MethodGet, "/api/rule-engine/points/abc/motion-thresholds", "", http.StatusBadRequest},
		{"punto sin versión", http.MethodGet, "/api/rule-engine/points/5/motion-thresholds", "", http.StatusNotFound},
		{"parámetro desconocido", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"amarillo": 0.2}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"null", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"jump_deg": null}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"entero con decimales", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"still_hours": 1.5}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"regla de la tabla", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"jump_deg": 0.05}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"abrupt_deg retirado", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"abrupt_deg": 0.05}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"fila sin amarillo_deg", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"rules": [{"window_hours": 3, "axis": "ANY"}]}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"fila con clave desconocida", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"rules": [{"window_hours": 3, "axis": "ANY", "amarillo_deg": 0.04, "x": 1}]}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"tabla vacía", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"rules": []}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"amarillo_deg 0,01", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"rules": [{"window_hours": 3, "axis": "ANY", "amarillo_deg": 0.01}]}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"send_naranja inválido", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
			`{"patch": {"send_naranja": "SI"}, "reason": "r", "created_by": "c"}`, http.StatusBadRequest},
		{"timeline sin rango", http.MethodGet, "/api/rule-engine/points/11556/motion-thresholds/timeline", "", http.StatusBadRequest},
		{"simulate sin rango", http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds/simulate",
			`{"patch": {}}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := call(mux, tc.method, tc.path, tc.body); rec.Code != tc.want {
				t.Fatalf("status %d, se esperaba %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestMotionHandler_Simulate(t *testing.T) {
	rec := call(newMotionServer(), http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds/simulate",
		`{"patch": {"send_naranja": "ACTIVE"}, "from": "2026-10-01T00:00:00Z", "to": "2026-10-02T00:00:00Z"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var r ruleEngineHTTP.MotionSimulationResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.HoursEvaluated != 24 || r.Params["send_naranja"] != "ACTIVE" || r.ComparedToCurrent.CurrentVersionID == 0 {
		t.Fatalf("simulación %+v", r)
	}
}

func TestParseMotionPatch_TypesEachParam(t *testing.T) {
	patch, err := ruleEngineHTTP.ParseMotionPatch(map[string]json.RawMessage{
		"jump_deg": json.RawMessage(`0.02`), "still_hours": json.RawMessage(`12`),
		"alerts_enabled": json.RawMessage(`false`), "temp_mode": json.RawMessage(`"SOLO_MOVIMIENTO"`),
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := patch.ApplyTo(entities.DefaultMotionParams())
	if err != nil || p.JumpDeg != 0.02 || p.StillHours != 12 || p.AlertsEnabled || p.TempMode != entities.TempModeMotionOnly {
		t.Fatalf("params %+v, err %v", p, err)
	}
	if _, err := ruleEngineHTTP.ParseMotionPatch(nil); !errors.Is(err, domain.ErrInvalidPatch) {
		t.Fatalf("sin patch: %v", err)
	}
}
