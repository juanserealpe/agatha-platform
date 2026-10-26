// Package http expone la API de Alert Management (/api/alerts). Sin
// autenticación (temporal, RN-10): las escrituras llevan origin y, desde el
// celular, device_identifier, client_record_id y recorded_at.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const maxBodyBytes = 64 << 10

// Rutas (Go 1.22 ServeMux). Las literales ganan a {id}.
const (
	RouteSearch        = "GET /api/alerts"
	RouteExport        = "GET /api/alerts/export.xlsx"
	RoutePending       = "GET /api/alerts/pending"
	RouteActive        = "GET /api/alerts/active"
	RouteDetail        = "GET /api/alerts/{id}"
	RouteDeviceHistory = "GET /api/alerts/devices/{id}/alerts"
	RouteClassify      = "POST /api/alerts/{id}/classification"
	RouteLabel         = "POST /api/alerts/{id}/label"
	RouteObservation   = "POST /api/alerts/{id}/observations"
	RoutePolicy        = "GET /api/alerts/policies"
	RouteCreatePolicy  = "POST /api/alerts/policies"
	RoutePolicyHistory = "GET /api/alerts/policies/history"
)

// Queries son las consultas (en producción, *services.QueryService).
type Queries interface {
	Search(ctx context.Context, p services.FilterParams, page int) (services.SearchPage, error)
	Export(ctx context.Context, p services.FilterParams, w io.Writer) error
	Pending(ctx context.Context, limit int) ([]ports.AlertSummary, int64, error)
	Active(ctx context.Context) ([]services.ActiveDevice, error)
	Detail(ctx context.Context, id int64) (*ports.AlertDetail, error)
	DeviceHistory(ctx context.Context, pointID int64, from, to time.Time) ([]ports.AlertDetail, error)
}

// Management es la gestión (en producción, *services.ManagementService).
type Management interface {
	Classify(ctx context.Context, cmd services.ClassifyCommand) (services.ManagementResult, error)
	Label(ctx context.Context, cmd services.LabelCommand) (services.ManagementResult, error)
	AddObservation(ctx context.Context, cmd services.ObservationCommand) (services.ManagementResult, error)
}

// Policies es la política versionada (en producción, *services.PolicyService).
type Policies interface {
	Current(ctx context.Context, pointID *int64, eventType entities.EventType) (entities.Policy, error)
	History(ctx context.Context, pointID *int64, eventType entities.EventType) ([]entities.Policy, error)
	Create(ctx context.Context, cmd services.CreatePolicyCommand) (entities.Policy, error)
}

// AlertHandler expone /api/alerts.
type AlertHandler struct {
	queries    Queries
	management Management
	policies   Policies
	log        *logger.Logger
}

func NewAlertHandler(queries Queries, management Management, policies Policies, log *logger.Logger) *AlertHandler {
	return &AlertHandler{queries: queries, management: management, policies: policies, log: log}
}

func (h *AlertHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteSearch, h.Search)
	mux.HandleFunc(RouteExport, h.Export)
	mux.HandleFunc(RoutePending, h.Pending)
	mux.HandleFunc(RouteActive, h.Active)
	mux.HandleFunc(RouteDetail, h.Detail)
	mux.HandleFunc(RouteDeviceHistory, h.DeviceHistory)
	mux.HandleFunc(RouteClassify, h.Classify)
	mux.HandleFunc(RouteLabel, h.Label)
	mux.HandleFunc(RouteObservation, h.Observation)
	mux.HandleFunc(RoutePolicy, h.Policy)
	mux.HandleFunc(RouteCreatePolicy, h.CreatePolicy)
	mux.HandleFunc(RoutePolicyHistory, h.PolicyHistory)
}

func filterParams(r *http.Request) (services.FilterParams, error) {
	q := r.URL.Query()
	p := services.FilterParams{From: q.Get("from"), To: q.Get("to"), MaxLevels: q["max_level"], Statuses: q["status"], Events: q["event"]}
	if raw := strings.TrimSpace(q.Get("device")); raw != "" && !strings.EqualFold(raw, "todos") {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return p, domain.ErrInvalidFilter
		}
		p.PointID = id
	}
	return p, nil
}

// Search: GET /api/alerts?device=&from=&to=&max_level=&status=&event=&page=
func (h *AlertHandler) Search(w http.ResponseWriter, r *http.Request) {
	p, err := filterParams(r)
	if err != nil {
		h.fail(w, "GET /api/alerts", err)
		return
	}
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		if page, err = strconv.Atoi(raw); err != nil || page < 1 {
			h.fail(w, "GET /api/alerts", domain.ErrInvalidFilter, "page", raw)
			return
		}
	}
	result, err := h.queries.Search(r.Context(), p, page)
	if err != nil {
		h.fail(w, "GET /api/alerts", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, SearchResponse{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Items: toItems(result.Items)})
}

// Export: GET /api/alerts/export.xlsx (mismos filtros). El archivo se arma en
// memoria: un error nunca deja un Excel a medias.
func (h *AlertHandler) Export(w http.ResponseWriter, r *http.Request) {
	p, err := filterParams(r)
	if err != nil {
		h.fail(w, "GET /api/alerts/export.xlsx", err)
		return
	}
	var buf bytes.Buffer
	if err := h.queries.Export(r.Context(), p, &buf); err != nil {
		h.fail(w, "GET /api/alerts/export.xlsx", err)
		return
	}
	name := "alertas_" + time.Now().In(time.FixedZone("COT", -5*3600)).Format("20060102_1504") + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// Pending: GET /api/alerts/pending?limit=5
func (h *AlertHandler) Pending(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 {
			h.fail(w, "GET /api/alerts/pending", domain.ErrInvalidFilter, "limit", raw)
			return
		}
	}
	items, total, err := h.queries.Pending(r.Context(), limit)
	if err != nil {
		h.fail(w, "GET /api/alerts/pending", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, PendingResponse{Total: total, Items: toItems(items)})
}

// Active: GET /api/alerts/active
func (h *AlertHandler) Active(w http.ResponseWriter, r *http.Request) {
	list, err := h.queries.Active(r.Context())
	if err != nil {
		h.fail(w, "GET /api/alerts/active", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toActive(list))
}

// Detail: GET /api/alerts/{id} (id numérico o ALR-000123).
func (h *AlertHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r, "GET /api/alerts/{id}")
	if !ok {
		return
	}
	d, err := h.queries.Detail(r.Context(), id)
	if err != nil {
		h.fail(w, "GET /api/alerts/{id}", err, "id", id)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toDetail(*d))
}

// DeviceHistory: GET /api/alerts/devices/{id}/alerts?from=&to= (fechas como el
// historial; sin fechas, los últimos 30 días).
func (h *AlertHandler) DeviceHistory(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("id")
	pointID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || pointID <= 0 {
		h.fail(w, "GET /api/alerts/devices/{id}/alerts", domain.ErrInvalidMeasuringPointID, "id", raw)
		return
	}
	f, err := services.ParseFilter(services.FilterParams{From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to")})
	if err != nil {
		h.fail(w, "GET /api/alerts/devices/{id}/alerts", err)
		return
	}
	list, err := h.queries.DeviceHistory(r.Context(), pointID, f.From, f.To)
	if err != nil {
		h.fail(w, "GET /api/alerts/devices/{id}/alerts", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toDeviceHistory(pointID, list))
}

// Classify: POST /api/alerts/{id}/classification
func (h *AlertHandler) Classify(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r, "POST classification")
	if !ok {
		return
	}
	var req ClassificationRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST classification", err)
		return
	}
	result, err := h.management.Classify(r.Context(), services.ClassifyCommand{
		AlertID: id, Classification: entities.Classification(strings.ToUpper(req.Classification)),
		Observation: req.Observation, ActionOrigin: req.toOrigin(),
	})
	h.managementResponse(w, "POST classification", id, result, err)
}

// Label: POST /api/alerts/{id}/label
func (h *AlertHandler) Label(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r, "POST label")
	if !ok {
		return
	}
	var req LabelRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST label", err)
		return
	}
	result, err := h.management.Label(r.Context(), services.LabelCommand{
		AlertID: id, Label: entities.Label(strings.ToUpper(req.Label)), OtherText: req.OtherText,
		Observation: req.Observation, ActionOrigin: req.toOrigin(),
	})
	h.managementResponse(w, "POST label", id, result, err)
}

// Observation: POST /api/alerts/{id}/observations
func (h *AlertHandler) Observation(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r, "POST observations")
	if !ok {
		return
	}
	var req ObservationRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST observations", err)
		return
	}
	result, err := h.management.AddObservation(r.Context(), services.ObservationCommand{AlertID: id, Text: req.Text, ActionOrigin: req.toOrigin()})
	h.managementResponse(w, "POST observations", id, result, err)
}

// managementResponse: 201 guardado · 200 ya recibido · 409 conflicto (guardado como observación).
func (h *AlertHandler) managementResponse(w http.ResponseWriter, endpoint string, id int64, result services.ManagementResult, err error) {
	if err != nil {
		h.fail(w, endpoint, err, "id", id)
		return
	}
	resp := ManagementResponse{Outcome: string(result.Outcome), Alert: toItem(ports.AlertSummary{Alert: result.Alert, PK: entities.DeviceOrDefault(result.Alert.MeasuringPointID, nil).PK})}
	status := http.StatusCreated
	switch result.Outcome {
	case services.OutcomeAlreadyReceived:
		status = http.StatusOK
	case services.OutcomeConflict:
		status = http.StatusConflict
		resp.Code = "CONFLICT"
		if e := result.Existing; e != nil {
			existing := &ExistingManagement{}
			if endpoint == "POST label" {
				existing.Label, existing.LabelOtherText = strOrNil(string(e.Label)), strOrNil(e.LabelOtherText)
				existing.LabeledAt, existing.Origin, existing.DeviceIdentifier = localPtr(e.LabeledAt), string(e.LabeledOrigin), strOrNil(e.LabeledDevice)
			} else {
				existing.Classification, existing.ClassifiedAt = strOrNil(string(e.Classification)), localPtr(e.ClassifiedAt)
				existing.Origin, existing.DeviceIdentifier = string(e.ClassifiedOrigin), strOrNil(e.ClassifiedDevice)
			}
			resp.Existing = existing
		}
	}
	sharedhttp.WriteJSON(w, status, resp)
}

func pointQuery(r *http.Request) (*int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("device"))
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, domain.ErrInvalidMeasuringPointID
	}
	return &id, nil
}

// Policy: GET /api/alerts/policies?device= (vigente: la del punto o la global).
func (h *AlertHandler) Policy(w http.ResponseWriter, r *http.Request) {
	point, err := pointQuery(r)
	if err != nil {
		h.fail(w, "GET policies", err)
		return
	}
	p, err := h.policies.Current(r.Context(), point, entities.EventType(r.URL.Query().Get("event_type")))
	if err != nil {
		h.fail(w, "GET policies", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toPolicy(p))
}

// PolicyHistory: GET /api/alerts/policies/history?device= (sin device = global).
func (h *AlertHandler) PolicyHistory(w http.ResponseWriter, r *http.Request) {
	point, err := pointQuery(r)
	if err != nil {
		h.fail(w, "GET policies/history", err)
		return
	}
	list, err := h.policies.History(r.Context(), point, entities.EventType(r.URL.Query().Get("event_type")))
	if err != nil {
		h.fail(w, "GET policies/history", err)
		return
	}
	out := make([]PolicyResponse, len(list))
	for i, p := range list {
		out[i] = toPolicy(p)
	}
	sharedhttp.WriteJSON(w, http.StatusOK, out)
}

// CreatePolicy: POST /api/alerts/policies
func (h *AlertHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req CreatePolicyRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST policies", err)
		return
	}
	cmd := services.CreatePolicyCommand{
		PointID: req.MeasuringPointID, EventType: entities.EventType(req.EventType), Patch: req.Patch.toPatch(),
		Reason: req.Reason, CreatedBy: req.CreatedBy,
	}
	if req.ValidFrom != nil {
		cmd.ValidFrom = *req.ValidFrom
	}
	p, err := h.policies.Create(r.Context(), cmd)
	if err != nil {
		h.fail(w, "POST policies", err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusCreated, toPolicy(p))
}

// ParseAlertID acepta 123 o ALR-000123.
func ParseAlertID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 4 && strings.EqualFold(raw[:4], "ALR-") {
		raw = raw[4:]
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}

func (h *AlertHandler) alertID(w http.ResponseWriter, r *http.Request, endpoint string) (int64, bool) {
	id, ok := ParseAlertID(r.PathValue("id"))
	if !ok {
		h.fail(w, endpoint, domain.ErrInvalidAlertID, "id", r.PathValue("id"))
	}
	return id, ok
}

func (h *AlertHandler) fail(w http.ResponseWriter, endpoint string, err error, fields ...any) {
	h.log.Warn("[ENDPOINT/ALERTS] "+endpoint+" - Solicitud rechazada", append(fields, "error", err)...)
	sharedhttp.HandleError(w, err)
}

// decode lee un JSON acotado, sin campos desconocidos ni datos extra.
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil || decoder.More() {
		return domain.ErrInvalidBody
	}
	return nil
}
