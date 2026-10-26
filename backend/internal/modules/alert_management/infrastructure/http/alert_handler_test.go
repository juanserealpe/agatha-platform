package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var now = time.Date(2026, 8, 24, 17, 35, 0, 0, time.UTC)

type noDevices struct{}

func (noDevices) Get(context.Context, int64) (*entities.Device, error) { return nil, nil }

type fakeQueries struct {
	detail     *ports.AlertDetail
	lastDetail int64
	filter     services.FilterParams
	exportErr  error
}

func (f *fakeQueries) Search(_ context.Context, p services.FilterParams, page int) (services.SearchPage, error) {
	f.filter = p
	if _, err := services.ParseFilter(p); err != nil {
		return services.SearchPage{}, err
	}
	return services.SearchPage{Total: 0, Page: page, PageSize: 10}, nil
}

func (f *fakeQueries) Export(_ context.Context, _ services.FilterParams, w io.Writer) error {
	if f.exportErr != nil {
		return f.exportErr
	}
	_, err := w.Write([]byte("PK\x03\x04"))
	return err
}

func (f *fakeQueries) Pending(context.Context, int) ([]ports.AlertSummary, int64, error) {
	return nil, 0, nil
}
func (f *fakeQueries) Active(context.Context) ([]services.ActiveDevice, error) { return nil, nil }

func (f *fakeQueries) Detail(_ context.Context, id int64) (*ports.AlertDetail, error) {
	f.lastDetail = id
	if f.detail == nil {
		return nil, domain.ErrAlertNotFound
	}
	return f.detail, nil
}

func (f *fakeQueries) DeviceHistory(context.Context, int64, time.Time, time.Time) ([]ports.AlertDetail, error) {
	return nil, nil
}

type server struct {
	mux     *http.ServeMux
	store   *memory.AlertStore
	queries *fakeQueries
	alertID int64
}

func newServer(t *testing.T) *server {
	t.Helper()
	log := logger.New("AlertManagementTest", "Error")
	clock := ports.ClockFunc(func() time.Time { return now })
	store := memory.NewAlertStore()
	policies := memory.NewPolicyStore()

	// Una alerta activa, creada por el procesamiento real.
	processor := services.NewProcessingService(store, policies, noDevices{}, nil, clock, log)
	v1, v2 := 0.17, 0.18
	for n, notice := range []entities.LevelNotice{
		{MeasuringPointID: 11556, EvaluatedAt: now.Add(-2 * time.Hour).Truncate(time.Hour), Status: entities.StatusCalculated, IndicatorValue: &v1,
			RawLevel: entities.LevelAmarillo, ComputedLevel: entities.LevelAmarillo, EpisodeEvent: entities.EpisodeNone},
		{MeasuringPointID: 11556, EvaluatedAt: now.Add(-time.Hour).Truncate(time.Hour), Status: entities.StatusCalculated, IndicatorValue: &v2,
			RawLevel: entities.LevelAmarillo, ComputedLevel: entities.LevelAmarillo, EpisodeEvent: entities.EpisodeOpened, InEpisode: true, HoursAbove: 2},
	} {
		if _, err := processor.Process(context.Background(), notice, entities.SourceLive); err != nil {
			t.Fatalf("hora %d: %v", n, err)
		}
	}

	queries := &fakeQueries{}
	handler := NewAlertHandler(queries,
		services.NewManagementService(store, clock, log),
		services.NewPolicyService(policies, clock, log),
		log)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux) // un conflicto de rutas haría panic acá
	return &server{mux: mux, store: store, queries: queries, alertID: store.Alerts()[0].ID}
}

func (s *server) do(method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("cuerpo no JSON (%d): %s", rec.Code, rec.Body.String())
	}
	return out
}

// Escenarios 16 y 18 por HTTP: 201, 409 con la vigente y 200 ALREADY_RECEIVED.
func TestClassification_HTTPFlow(t *testing.T) {
	s := newServer(t)
	rec := s.do("POST", "/api/alerts/ALR-000123/classification",
		`{"classification":"FALSA_ALERTA","observation":"Recorrido PK31-PK37 sin hallazgos","origin":"APP_MOVIL",
		  "device_identifier":"Celular cuadrilla 1","client_record_id":"6f1c0e3a-1b2c-4d5e-8f90-1234567890ab","recorded_at":"2026-08-24T12:10:00-05:00"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	alert := body["alert"].(map[string]any)
	if body["outcome"] != "SAVED" || alert["management_status"] != "CLASIFICADA_SIN_ETIQUETA" || alert["state"] != "ACTIVA" {
		t.Fatalf("cuerpo = %v", body)
	}
	if acts := alert["available_actions"].([]any); len(acts) != 2 || acts[0] != "ADD_LABEL" {
		t.Fatalf("acciones = %v", acts)
	}

	rec = s.do("POST", "/api/alerts/123/classification", `{"classification":"CONFIRMADA","origin":"WEB"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflicto: status = %d: %s", rec.Code, rec.Body.String())
	}
	body = decodeBody(t, rec)
	if body["outcome"] != "SAVED_AS_CONFLICT_OBSERVATION" || body["existing"].(map[string]any)["classification"] != "FALSA_ALERTA" {
		t.Fatalf("cuerpo = %v", body)
	}

	rec = s.do("POST", "/api/alerts/123/classification",
		`{"classification":"FALSA_ALERTA","origin":"APP_MOVIL","device_identifier":"Celular cuadrilla 1","client_record_id":"6f1c0e3a-1b2c-4d5e-8f90-1234567890ab"}`)
	if rec.Code != http.StatusOK || decodeBody(t, rec)["outcome"] != "ALREADY_RECEIVED" {
		t.Fatalf("reenvío: status = %d: %s", rec.Code, rec.Body.String())
	}
}

// Escenario 17 por HTTP.
func TestLabel_HTTPRejections(t *testing.T) {
	s := newServer(t)
	if rec := s.do("POST", "/api/alerts/123/label", `{"label":"CLIMA_EXTREMO","origin":"WEB"}`); rec.Code != http.StatusConflict {
		t.Fatalf("sin clasificar: status = %d", rec.Code)
	}
	s.do("POST", "/api/alerts/123/classification", `{"classification":"CONFIRMADA","origin":"WEB"}`)
	if rec := s.do("POST", "/api/alerts/123/label", `{"label":"CLIMA_EXTREMO","origin":"WEB"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("otra lista: status = %d", rec.Code)
	}
	if rec := s.do("POST", "/api/alerts/123/label", `{"label":"OTRO","origin":"WEB"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("OTRO sin texto: status = %d", rec.Code)
	}
	if rec := s.do("POST", "/api/alerts/123/label", `{"label":"OTRO","other_text":"Prueba hidrostática","origin":"WEB"}`); rec.Code != http.StatusCreated {
		t.Fatalf("OTRO con texto: status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWrites_RejectUnknownFieldsAndBadIDs(t *testing.T) {
	s := newServer(t)
	if rec := s.do("POST", "/api/alerts/123/classification", `{"classification":"CONFIRMADA","origin":"WEB","photo":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("campo desconocido: status = %d", rec.Code)
	}
	if rec := s.do("POST", "/api/alerts/abc/classification", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("id inválido: status = %d", rec.Code)
	}
	if rec := s.do("POST", "/api/alerts/999/observations", `{"text":"hola","origin":"WEB"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("inexistente: status = %d", rec.Code)
	}
}

func TestRoutes_LiteralsWinOverID(t *testing.T) {
	s := newServer(t)
	if rec := s.do("GET", "/api/alerts/pending", ""); rec.Code != http.StatusOK {
		t.Fatalf("pending: %d %s", rec.Code, rec.Body.String())
	}
	if rec := s.do("GET", "/api/alerts/ALR-000123", ""); rec.Code != http.StatusNotFound || s.queries.lastDetail != 123 {
		t.Fatalf("detalle por código: %d, id = %d", rec.Code, s.queries.lastDetail)
	}
	if rec := s.do("GET", "/api/alerts?device=todos&from=31/08/2026&to=01/08/2026", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("fin antes de inicio: status = %d", rec.Code)
	}
	if rec := s.do("GET", "/api/alerts?device=11556&max_level=NARANJA,ROJO&page=2", ""); rec.Code != http.StatusOK || s.queries.filter.PointID != 11556 {
		t.Fatalf("búsqueda: %d %s", rec.Code, rec.Body.String())
	}
}

func TestExport_HeadersAndTooLarge(t *testing.T) {
	s := newServer(t)
	rec := s.do("GET", "/api/alerts/export.xlsx?device=11556", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), ".xlsx") ||
		rec.Header().Get("Content-Type") != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("export: %d %v", rec.Code, rec.Header())
	}
	s.queries.exportErr = domain.ErrExportTooLarge
	rec = s.do("GET", "/api/alerts/export.xlsx", "")
	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Header().Get("Content-Type"), "spreadsheet") {
		t.Fatalf("más de 10.000: %d %v", rec.Code, rec.Header())
	}
}

func TestPolicies_HTTP(t *testing.T) {
	s := newServer(t)
	rec := s.do("GET", "/api/alerts/policies", "")
	if rec.Code != http.StatusOK || decodeBody(t, rec)["downgrade_hours"] != float64(6) {
		t.Fatalf("vigente: %d %s", rec.Code, rec.Body.String())
	}
	rec = s.do("POST", "/api/alerts/policies", `{"measuring_point_id":11556,"patch":{"downgrade_hours":4},"reason":"piloto","created_by":"jvarona"}`)
	if rec.Code != http.StatusCreated || decodeBody(t, rec)["downgrade_hours"] != float64(4) {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body.String())
	}
	if rec := s.do("POST", "/api/alerts/policies", `{"patch":{"nope":1},"reason":"x","created_by":"y"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("campo desconocido en el parche: %d", rec.Code)
	}
}

func TestParseAlertID(t *testing.T) {
	for raw, want := range map[string]int64{"123": 123, "ALR-000123": 123, "alr-1234567": 1234567} {
		if got, ok := ParseAlertID(raw); !ok || got != want {
			t.Errorf("ParseAlertID(%q) = %d, %v", raw, got, ok)
		}
	}
	for _, raw := range []string{"", "ALR-", "0", "-5", "ALR-x"} {
		if _, ok := ParseAlertID(raw); ok {
			t.Errorf("ParseAlertID(%q) debía fallar", raw)
		}
	}
}

// Alert Management no entrega avisos: no hay feed web ni registro de celulares
// (los expone el módulo de notificaciones).
func TestRoutes_NoNotificationDelivery(t *testing.T) {
	s := newServer(t)
	if rec := s.do("POST", "/api/alerts/push-devices", `{}`); rec.Code == http.StatusOK {
		t.Fatalf("push-devices: %d", rec.Code)
	}
	if rec := s.do("GET", "/api/alerts/notifications", ""); rec.Code == http.StatusOK {
		t.Fatalf("notifications: %d", rec.Code)
	}
}
