package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// maxBodyBytes acota el cuerpo de los POST (un parche y una lista de puntos).
const maxBodyBytes = 64 << 10

// ThresholdUseCases son los casos de uso de umbrales que expone el handler
// (en producción, *services.ThresholdService).
type ThresholdUseCases interface {
	GetCurrentThresholds(ctx context.Context, pointID int64, eventType entities.EventType) (entities.ThresholdVersion, error)
	GetThresholdHistory(ctx context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error)
	GetThresholdTimeline(ctx context.Context, pointID int64, eventType entities.EventType, from, to time.Time) ([]entities.ThresholdSegment, error)
	CreateThresholdVersion(ctx context.Context, cmd services.CreateThresholdVersionCommand) (entities.ThresholdVersion, error)
	CreateThresholdVersionBulk(ctx context.Context, cmd services.CreateThresholdVersionBulkCommand) ([]entities.ThresholdVersion, error)
}

// Simulator es la simulación de umbrales (en producción, *services.SimulationService).
type Simulator interface {
	Simulate(ctx context.Context, cmd services.SimulateCommand) (services.SimulationResult, error)
}

// ThresholdHandler expone los umbrales versionados y la simulación.
type ThresholdHandler struct {
	thresholds ThresholdUseCases
	simulator  Simulator
	log        *logger.Logger
}

func NewThresholdHandler(thresholds ThresholdUseCases, simulator Simulator, log *logger.Logger) *ThresholdHandler {
	return &ThresholdHandler{thresholds: thresholds, simulator: simulator, log: log}
}

// Rutas (Go 1.22 ServeMux). Las registra internal/bootstrap.
const (
	RouteCurrent  = "GET /api/rule-engine/points/{id}/thresholds"
	RouteHistory  = "GET /api/rule-engine/points/{id}/thresholds/history"
	RouteTimeline = "GET /api/rule-engine/points/{id}/thresholds/timeline"
	RouteCreate   = "POST /api/rule-engine/points/{id}/thresholds"
	RouteBulk     = "POST /api/rule-engine/thresholds/bulk"
	RouteSimulate = "POST /api/rule-engine/points/{id}/thresholds/simulate"
)

// RegisterRoutes registra los endpoints de umbrales en el mux.
func (h *ThresholdHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteCurrent, h.GetCurrent)
	mux.HandleFunc(RouteHistory, h.GetHistory)
	mux.HandleFunc(RouteTimeline, h.GetTimeline)
	mux.HandleFunc(RouteCreate, h.Create)
	mux.HandleFunc(RouteBulk, h.CreateBulk)
	mux.HandleFunc(RouteSimulate, h.Simulate)
}

// GetCurrent devuelve la versión vigente ahora. Query: event_type (opcional, FUGA).
func (h *ThresholdHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.pointID(w, r, "GET /thresholds")
	if !ok {
		return
	}

	v, err := h.thresholds.GetCurrentThresholds(r.Context(), pointID, eventType(r))
	if err != nil {
		h.fail(w, "GET /thresholds", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toVersionResponse(v))
}

// GetHistory devuelve todas las versiones, ascendentes, con autor y motivo.
func (h *ThresholdHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.pointID(w, r, "GET /thresholds/history")
	if !ok {
		return
	}

	history, err := h.thresholds.GetThresholdHistory(r.Context(), pointID, eventType(r))
	if err != nil {
		h.fail(w, "GET /thresholds/history", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toVersionsResponse(history))
}

// GetTimeline devuelve los tramos de vigencia recortados a [from, to) (RFC3339, obligatorios).
func (h *ThresholdHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.pointID(w, r, "GET /thresholds/timeline")
	if !ok {
		return
	}

	query := r.URL.Query()
	from, errFrom := time.Parse(time.RFC3339, query.Get("from"))
	to, errTo := time.Parse(time.RFC3339, query.Get("to"))
	if errFrom != nil || errTo != nil {
		h.fail(w, "GET /thresholds/timeline", domain.ErrInvalidTimeRange, "from", query.Get("from"), "to", query.Get("to"))
		return
	}

	segments, err := h.thresholds.GetThresholdTimeline(r.Context(), pointID, eventType(r), from.UTC(), to.UTC())
	if err != nil {
		h.fail(w, "GET /thresholds/timeline", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toTimelineResponse(segments))
}

// Create crea una versión nueva del punto con los campos del parche.
func (h *ThresholdHandler) Create(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.pointID(w, r, "POST /thresholds")
	if !ok {
		return
	}

	var req CreateVersionRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST /thresholds", err, "measuring_point_id", pointID)
		return
	}
	patch, err := ParsePatch(req.Patch)
	if err != nil {
		h.fail(w, "POST /thresholds", err, "measuring_point_id", pointID)
		return
	}

	v, err := h.thresholds.CreateThresholdVersion(r.Context(), services.CreateThresholdVersionCommand{
		PointID:   pointID,
		EventType: entities.EventType(req.EventType),
		Patch:     patch,
		ValidFrom: timeOrZero(req.ValidFrom),
		Reason:    req.Reason,
		CreatedBy: req.CreatedBy,
	})
	if err != nil {
		h.fail(w, "POST /thresholds", err, "measuring_point_id", pointID)
		return
	}

	h.log.Info("[ENDPOINT/RULE_ENGINE] POST /thresholds - Versión creada",
		"measuring_point_id", pointID, "version", v.Version, "created_by", v.CreatedBy)
	sharedhttp.WriteJSON(w, http.StatusCreated, toVersionResponse(v))
}

// CreateBulk crea la misma versión en varios puntos, todo o nada. point_ids es obligatorio.
func (h *ThresholdHandler) CreateBulk(w http.ResponseWriter, r *http.Request) {
	var req BulkCreateVersionRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST /thresholds/bulk", err)
		return
	}
	// La API no aplica "todos los puntos" implícito: una lista vacía es un error.
	if len(req.PointIDs) == 0 {
		h.fail(w, "POST /thresholds/bulk", domain.ErrNoPoints)
		return
	}
	patch, err := ParsePatch(req.Patch)
	if err != nil {
		h.fail(w, "POST /thresholds/bulk", err)
		return
	}

	created, err := h.thresholds.CreateThresholdVersionBulk(r.Context(), services.CreateThresholdVersionBulkCommand{
		PointIDs:  req.PointIDs,
		EventType: entities.EventType(req.EventType),
		Patch:     patch,
		ValidFrom: timeOrZero(req.ValidFrom),
		Reason:    req.Reason,
		CreatedBy: req.CreatedBy,
	})
	if err != nil {
		h.fail(w, "POST /thresholds/bulk", err, "points", req.PointIDs)
		return
	}

	h.log.Info("[ENDPOINT/RULE_ENGINE] POST /thresholds/bulk - Versiones creadas", "points", len(created), "created_by", req.CreatedBy)
	sharedhttp.WriteJSON(w, http.StatusCreated, toVersionsResponse(created))
}

// Simulate corre el histórico con los valores propuestos, sin guardar ni publicar.
func (h *ThresholdHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.pointID(w, r, "POST /thresholds/simulate")
	if !ok {
		return
	}

	var req SimulateRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, "POST /thresholds/simulate", err, "measuring_point_id", pointID)
		return
	}
	if req.From == nil || req.To == nil {
		h.fail(w, "POST /thresholds/simulate", domain.ErrInvalidTimeRange, "measuring_point_id", pointID)
		return
	}
	patch, err := ParsePatch(req.Patch)
	if err != nil {
		h.fail(w, "POST /thresholds/simulate", err, "measuring_point_id", pointID)
		return
	}

	result, err := h.simulator.Simulate(r.Context(), services.SimulateCommand{
		PointID:   pointID,
		EventType: entities.EventType(req.EventType),
		Patch:     patch,
		From:      *req.From,
		To:        *req.To,
	})
	if err != nil {
		h.fail(w, "POST /thresholds/simulate", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toSimulationResponse(result))
}

func (h *ThresholdHandler) pointID(w http.ResponseWriter, r *http.Request, endpoint string) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		h.fail(w, endpoint, domain.ErrInvalidMeasuringPointID, "id", raw)
		return 0, false
	}
	return id, true
}

func (h *ThresholdHandler) fail(w http.ResponseWriter, endpoint string, err error, fields ...any) {
	h.log.Warn("[ENDPOINT/RULE_ENGINE] "+endpoint+" - Solicitud rechazada", append(fields, "error", err)...)
	sharedhttp.HandleError(w, err)
}

// decode lee un JSON acotado, sin campos desconocidos ni datos extra.
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return domain.ErrInvalidBody
	}
	if decoder.More() {
		return domain.ErrInvalidBody
	}
	return nil
}

func eventType(r *http.Request) entities.EventType {
	return entities.EventType(r.URL.Query().Get("event_type"))
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
