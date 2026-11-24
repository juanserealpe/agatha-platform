package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	ruleEngineHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const pointID int64 = 11556

var (
	now  = time.Date(2026, 10, 24, 2, 40, 0, 0, time.UTC)
	day1 = time.Date(2026, 7, 8, 5, 0, 0, 0, time.UTC)
)

// emptySource: feature_evaluations sin horas (la simulación devuelve 0 episodios).
type emptySource struct{}

func (emptySource) ListPoints(context.Context, time.Time, time.Time) ([]int64, error) {
	return nil, nil
}
func (emptySource) ListCalculated(context.Context, int64, time.Time, time.Time, time.Time, int) ([]ports.CalculatedIndicator, error) {
	return nil, nil
}

type server struct {
	repo *memory.ThresholdRepository
	mux  *http.ServeMux
}

func newServer() *server {
	log := logger.New("RuleEngineHTTPTest", "Error")
	clock := ports.ClockFunc(func() time.Time { return now })
	repo := memory.NewThresholdRepository(entities.ThresholdVersion{
		MeasuringPointID: pointID, EventType: entities.EventTypeLeak, Version: 1,
		ThresholdParams: entities.DefaultThresholdParams(), ValidFrom: day1,
		Reason: entities.SeedReason, CreatedBy: entities.SystemCreatedBy, CreatedAt: day1,
	})
	sandbox := func(seed entities.ThresholdVersion) (repositories.ThresholdRepository, repositories.RuleEvaluationRepository) {
		return memory.NewThresholdRepository(seed), memory.NewRuleEvaluationRepository()
	}

	mux := http.NewServeMux()
	ruleEngineHTTP.NewThresholdHandler(
		services.NewThresholdService(repo, clock, log),
		services.NewSimulationService(repo, emptySource{}, sandbox, clock, log),
		log,
	).RegisterRoutes(mux)
	return &server{repo: repo, mux: mux}
}

func (s *server) do(t *testing.T, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	var out map[string]any
	if strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "{") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("respuesta no JSON: %s", rec.Body.String())
		}
	}
	return rec.Code, out
}

// Caso 1 (y ejemplo 5.3): amarillo a 0,16 con la v1 vigente.
func TestCreate_AmarilloV2(t *testing.T) {
	s := newServer()
	code, body := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds", `{
		"event_type": "FUGA", "patch": {"amarillo": 0.16}, "valid_from": null,
		"reason": "Amarillo a 0,16", "created_by": "jvarona@unicauca.edu.co"}`)

	if code != http.StatusCreated {
		t.Fatalf("status = %d, body = %v", code, body)
	}
	exit := body["exit"].(map[string]any)
	if body["version"].(float64) != 2 || body["previous_version_id"].(float64) != 1 || body["amarillo"].(float64) != 0.16 ||
		body["naranja"].(float64) != 0.25 || exit["amarillo"].(float64) != 0.14 || body["valid_to"] != nil ||
		body["persistence_mode"] != "STRICT" || body["created_by"] != "jvarona@unicauca.edu.co" {
		t.Fatalf("respuesta = %v", body)
	}

	versions := s.repo.All()
	if len(versions) != 2 || versions[0].ValidTo == nil || !versions[0].ValidTo.Equal(versions[1].ValidFrom) {
		t.Fatalf("v1 debe cerrarse en el valid_from de v2: %+v", versions)
	}

	code, current := s.do(t, http.MethodGet, "/api/rule-engine/points/11556/thresholds?event_type=FUGA", "")
	if code != http.StatusOK || current["version"].(float64) != 2 || current["previous_version_id"].(float64) != 1 {
		t.Fatalf("vigente = %d %v", code, current)
	}
}

func TestCreate_NullDisablesEscalation(t *testing.T) {
	s := newServer()
	code, body := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds",
		`{"patch": {"escalate_naranja_after_hours": null}, "reason": "r", "created_by": "c"}`)
	if code != http.StatusCreated || body["escalate_naranja_after_hours"] != nil || body["escalate_amarillo_after_hours"].(float64) != 12 {
		t.Fatalf("status = %d, body = %v", code, body)
	}
}

func TestCreate_Window(t *testing.T) {
	s := newServer()
	code, body := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds", `{"patch": {
		"persistence_mode": "WINDOW", "persistence_window_hours": 24, "persistence_min_hours": 18},
		"reason": "r", "created_by": "c"}`)
	if code != http.StatusCreated || body["persistence_mode"] != "WINDOW" {
		t.Fatalf("status = %d, body = %v", code, body)
	}
}

// Casos 2 y 3 más las validaciones del cuerpo: 400 sin escribir nada.
func TestCreate_Rejections(t *testing.T) {
	past := now.Add(-time.Hour).Format(time.RFC3339)
	tests := []struct {
		name, body string
	}{
		{"valid_from pasado", `{"patch": {"amarillo": 0.16}, "valid_from": "` + past + `", "reason": "r", "created_by": "c"}`},
		{"amarillo >= naranja", `{"patch": {"amarillo": 0.26}, "reason": "r", "created_by": "c"}`},
		{"sin reason", `{"patch": {"amarillo": 0.16}, "created_by": "c"}`},
		{"sin created_by", `{"patch": {"amarillo": 0.16}, "reason": "r"}`},
		{"sin patch", `{"reason": "r", "created_by": "c"}`},
		{"campo de patch desconocido", `{"patch": {"amarilo": 0.16}, "reason": "r", "created_by": "c"}`},
		{"null no anulable", `{"patch": {"amarillo": null}, "reason": "r", "created_by": "c"}`},
		{"tipo inválido", `{"patch": {"min_hours": 2.5}, "reason": "r", "created_by": "c"}`},
		{"campo raíz desconocido", `{"patch": {}, "reason": "r", "created_by": "c", "extra": 1}`},
		{"campos de planitud retirados", `{"patch": {"confirm_naranja_with_planitud": true, "d_pl_min": 12}, "reason": "r", "created_by": "c"}`},
		{"WINDOW incompleto", `{"patch": {"persistence_mode": "WINDOW"}, "reason": "r", "created_by": "c"}`},
		{"JSON roto", `{"patch":`},
	}
	for _, tt := range tests {
		s := newServer()
		code, body := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds", tt.body)
		if code != http.StatusBadRequest || body["code"] != "INVALID_INPUT" {
			t.Errorf("%s: status = %d, body = %v; se esperaba 400 INVALID_INPUT", tt.name, code, body)
		}
		if len(s.repo.All()) != 1 {
			t.Errorf("%s: no debía escribirse ninguna versión", tt.name)
		}
	}
}

// Caso 4: el segundo POST concurrente recibe 409.
func TestCreate_ConflictIs409(t *testing.T) {
	s := newServer()
	s.repo.FailNextTransaction = fmt.Errorf("55P03: %w", repositories.ErrConflict)

	code, body := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds",
		`{"patch": {"amarillo": 0.16}, "reason": "r", "created_by": "c"}`)
	if code != http.StatusConflict || body["code"] != "CONFLICT" {
		t.Fatalf("status = %d, body = %v; se esperaba 409 CONFLICT", code, body)
	}
}

func TestBulk(t *testing.T) {
	s := newServer()
	code, _ := s.do(t, http.MethodPost, "/api/rule-engine/thresholds/bulk",
		`{"point_ids": [11556, 11557], "patch": {"amarillo": 0.16}, "reason": "r", "created_by": "c"}`)
	if code != http.StatusCreated || len(s.repo.All()) != 3 {
		t.Fatalf("status = %d, versiones = %d; se esperaban 201 y 3", code, len(s.repo.All()))
	}

	code, _ = s.do(t, http.MethodPost, "/api/rule-engine/thresholds/bulk", `{"point_ids": [], "patch": {}, "reason": "r", "created_by": "c"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("point_ids vacío: status = %d, se esperaba 400", code)
	}
}

func TestGet_NotFoundAndInvalidID(t *testing.T) {
	s := newServer()
	if code, _ := s.do(t, http.MethodGet, "/api/rule-engine/points/999/thresholds", ""); code != http.StatusNotFound {
		t.Fatalf("punto sin umbrales: status = %d, se esperaba 404", code)
	}
	if code, _ := s.do(t, http.MethodGet, "/api/rule-engine/points/abc/thresholds", ""); code != http.StatusBadRequest {
		t.Fatalf("id inválido: status = %d, se esperaba 400", code)
	}
}

func TestHistoryAndTimeline(t *testing.T) {
	s := newServer()
	s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds", `{"patch": {"amarillo": 0.16}, "reason": "r", "created_by": "c"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/rule-engine/points/11556/thresholds/history", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	var history []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil || len(history) != 2 ||
		history[0]["previous_version_id"] != nil || history[1]["previous_version_id"].(float64) != 1 {
		t.Fatalf("historial = %s", rec.Body.String())
	}

	from, to := day1.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	req = httptest.NewRequest(http.MethodGet, "/api/rule-engine/points/11556/thresholds/timeline?from="+from+"&to="+to, nil)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	var timeline []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &timeline); err != nil || rec.Code != http.StatusOK || len(timeline) != 2 {
		t.Fatalf("línea de tiempo = %d %s", rec.Code, rec.Body.String())
	}

	if code, _ := s.do(t, http.MethodGet, "/api/rule-engine/points/11556/thresholds/timeline?from="+to+"&to="+from, ""); code != http.StatusBadRequest {
		t.Fatalf("rango invertido: status = %d, se esperaba 400", code)
	}
	if code, _ := s.do(t, http.MethodGet, "/api/rule-engine/points/11556/thresholds/timeline", ""); code != http.StatusBadRequest {
		t.Fatalf("sin rango: status = %d, se esperaba 400", code)
	}
}

func TestSimulate(t *testing.T) {
	s := newServer()
	code, body := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds/simulate",
		`{"patch": {"amarillo": 0.16}, "from": "2026-07-08T00:00:00Z", "to": "2026-10-22T00:00:00Z"}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body = %v", code, body)
	}
	summary := body["summary"].(map[string]any)
	byLevel := summary["by_max_level"].(map[string]any)
	thresholds := body["thresholds"].(map[string]any)
	if body["hours_evaluated"].(float64) != 0 || summary["episodes"].(float64) != 0 || byLevel["ROJO"].(float64) != 0 ||
		thresholds["amarillo"].(float64) != 0.16 || body["compared_to_current"] == nil {
		t.Fatalf("respuesta = %v", body)
	}
	if len(s.repo.All()) != 1 {
		t.Fatal("simulate no escribe versiones")
	}

	if code, _ := s.do(t, http.MethodPost, "/api/rule-engine/points/11556/thresholds/simulate", `{"patch": {}}`); code != http.StatusBadRequest {
		t.Fatalf("sin rango: status = %d, se esperaba 400", code)
	}
}

func TestParsePatch_AbsentVsNull(t *testing.T) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"escalate_amarillo_after_hours": 6, "persistence_window_hours": null}`), &raw); err != nil {
		t.Fatal(err)
	}
	patch, err := ruleEngineHTTP.ParsePatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !patch.EscalateAmarilloAfterHours.Set || *patch.EscalateAmarilloAfterHours.Value != 6 ||
		!patch.PersistenceWindowHours.Set || patch.PersistenceWindowHours.Value != nil || patch.EscalateNaranjaAfterHours.Set {
		t.Fatalf("parche = %+v", patch)
	}
	if _, err := ruleEngineHTTP.ParsePatch(nil); !errors.Is(err, domain.ErrInvalidPatch) {
		t.Fatalf("patch ausente: err = %v, se esperaba ErrInvalidPatch", err)
	}
}
