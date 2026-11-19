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
		v.Params["alerts_enabled"] != true || len(v.Params) != len(entities.MotionParamFields()) {
		t.Fatalf("respuesta %+v", v)
	}
}

func TestMotionHandler_CreateAndHistory(t *testing.T) {
	mux := newMotionServer()
	rec := call(mux, http.MethodPost, "/api/rule-engine/points/11556/motion-thresholds",
		`{"patch": {"abrupt_deg": 0.05, "temp_mode": "SOLO_MOVIMIENTO"}, "reason": "calibración", "created_by": "ana"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var v ruleEngineHTTP.MotionVersionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Version != 2 || v.Params["abrupt_deg"] != 0.05 || v.Params["temp_mode"] != "SOLO_MOVIMIENTO" || v.PreviousVersionID == nil {
		t.Fatalf("versión %+v", v)
	}

	rec = call(mux, http.MethodGet, "/api/rule-engine/points/11556/motion-thresholds/history", "")
	var history []ruleEngineHTTP.MotionVersionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &history)
	if rec.Code != http.StatusOK || len(history) != 2 || history[0].ValidTo == nil {
		t.Fatalf("historial %d: %s", rec.Code, rec.Body)
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
		`{"patch": {"abrupt_deg": 0.05}, "from": "2026-10-01T00:00:00Z", "to": "2026-10-02T00:00:00Z"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var r ruleEngineHTTP.MotionSimulationResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.HoursEvaluated != 24 || r.Params["abrupt_deg"] != 0.05 || r.ComparedToCurrent.CurrentVersionID == 0 {
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
