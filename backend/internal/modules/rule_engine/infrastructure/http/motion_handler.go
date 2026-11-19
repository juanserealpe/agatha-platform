package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// MotionThresholdUseCases son los casos de uso de motion_threshold_versions
// (en producción, *services.MotionThresholdService).
type MotionThresholdUseCases interface {
	GetCurrent(ctx context.Context, pointID int64) (entities.MotionThresholdVersion, error)
	GetHistory(ctx context.Context, pointID int64) ([]entities.MotionThresholdVersion, error)
	CreateVersion(ctx context.Context, cmd services.CreateMotionThresholdVersionCommand) (entities.MotionThresholdVersion, error)
}

// MotionSimulator es la simulación de movimiento (*services.MotionSimulationService).
type MotionSimulator interface {
	Simulate(ctx context.Context, cmd services.MotionSimulateCommand) (services.MotionSimulationResult, error)
}

// Rutas de movimiento (§6.4).
const (
	RouteMotionCurrent  = "GET /api/rule-engine/points/{id}/motion-thresholds"
	RouteMotionHistory  = "GET /api/rule-engine/points/{id}/motion-thresholds/history"
	RouteMotionCreate   = "POST /api/rule-engine/points/{id}/motion-thresholds"
	RouteMotionSimulate = "POST /api/rule-engine/points/{id}/motion-thresholds/simulate"
)

// MotionHandler expone las versiones de movimiento y su simulación. Reusa la
// validación, el decode y el manejo de errores del ThresholdHandler.
type MotionHandler struct {
	thresholds MotionThresholdUseCases
	simulator  MotionSimulator
	base       *ThresholdHandler
}

func NewMotionHandler(thresholds MotionThresholdUseCases, simulator MotionSimulator, log *logger.Logger) *MotionHandler {
	return &MotionHandler{thresholds: thresholds, simulator: simulator, base: &ThresholdHandler{log: log}}
}

func (h *MotionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteMotionCurrent, h.GetCurrent)
	mux.HandleFunc(RouteMotionHistory, h.GetHistory)
	mux.HandleFunc(RouteMotionCreate, h.Create)
	mux.HandleFunc(RouteMotionSimulate, h.Simulate)
}

// CreateMotionVersionRequest es el cuerpo de POST .../motion-thresholds.
// CreatedBy es TEMPORAL (sin autenticación todavía), como en FUGA.
type CreateMotionVersionRequest struct {
	Patch     map[string]json.RawMessage `json:"patch"`
	ValidFrom *time.Time                 `json:"valid_from"`
	Reason    string                     `json:"reason"`
	CreatedBy string                     `json:"created_by"`
}

// MotionSimulateRequest es el cuerpo de POST .../motion-thresholds/simulate.
type MotionSimulateRequest struct {
	Patch map[string]json.RawMessage `json:"patch"`
	From  *time.Time                 `json:"from"`
	To    *time.Time                 `json:"to"`
}

// MotionVersionResponse es una versión de movimiento. Params lleva los
// parámetros de §5.2 por nombre.
type MotionVersionResponse struct {
	ID                int64          `json:"id"`
	Version           int            `json:"version"`
	PreviousVersionID *int64         `json:"previous_version_id"`
	MeasuringPointID  int64          `json:"measuring_point_id"`
	Params            map[string]any `json:"params"`
	ValidFrom         time.Time      `json:"valid_from"`
	ValidTo           *time.Time     `json:"valid_to"`
	Reason            string         `json:"reason"`
	CreatedBy         string         `json:"created_by"`
	CreatedAt         time.Time      `json:"created_at"`
}

// TempStableResponse es el tramo de temperatura estable de un episodio.
type TempStableResponse struct {
	From *time.Time `json:"from"`
	To   *time.Time `json:"to"`
}

// SimulatedMotionEpisodeResponse es un episodio de la simulación.
type SimulatedMotionEpisodeResponse struct {
	OpenedAt   time.Time          `json:"opened_at"`
	ClearedAt  *time.Time         `json:"cleared_at"`
	Reasons    []string           `json:"reasons"`
	TempStable TempStableResponse `json:"temp_stable"`
}

// MotionSimulationResponse es la respuesta de simulate.
type MotionSimulationResponse struct {
	MeasuringPointID  int64                            `json:"measuring_point_id"`
	From              time.Time                        `json:"from"`
	To                time.Time                        `json:"to"`
	Params            map[string]any                   `json:"params"`
	HoursEvaluated    int                              `json:"hours_evaluated"`
	Episodes          []SimulatedMotionEpisodeResponse `json:"episodes"`
	ComparedToCurrent struct {
		CurrentVersionID int64                            `json:"current_version_id"`
		CurrentEpisodes  []SimulatedMotionEpisodeResponse `json:"current_episodes"`
		NewEpisodes      int                              `json:"new_episodes"`
		RemovedEpisodes  int                              `json:"removed_episodes"`
	} `json:"compared_to_current"`
}

func (h *MotionHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.base.pointID(w, r, "GET /motion-thresholds")
	if !ok {
		return
	}
	v, err := h.thresholds.GetCurrent(r.Context(), pointID)
	if err != nil {
		h.base.fail(w, "GET /motion-thresholds", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toMotionVersionResponse(v))
}

func (h *MotionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.base.pointID(w, r, "GET /motion-thresholds/history")
	if !ok {
		return
	}
	history, err := h.thresholds.GetHistory(r.Context(), pointID)
	if err != nil {
		h.base.fail(w, "GET /motion-thresholds/history", err, "measuring_point_id", pointID)
		return
	}
	out := make([]MotionVersionResponse, len(history))
	for i, v := range history {
		out[i] = toMotionVersionResponse(v)
	}
	sharedhttp.WriteJSON(w, http.StatusOK, out)
}

func (h *MotionHandler) Create(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.base.pointID(w, r, "POST /motion-thresholds")
	if !ok {
		return
	}
	var req CreateMotionVersionRequest
	if err := decode(w, r, &req); err != nil {
		h.base.fail(w, "POST /motion-thresholds", err, "measuring_point_id", pointID)
		return
	}
	patch, err := ParseMotionPatch(req.Patch)
	if err != nil {
		h.base.fail(w, "POST /motion-thresholds", err, "measuring_point_id", pointID)
		return
	}
	v, err := h.thresholds.CreateVersion(r.Context(), services.CreateMotionThresholdVersionCommand{
		PointID: pointID, Patch: patch, ValidFrom: timeOrZero(req.ValidFrom), Reason: req.Reason, CreatedBy: req.CreatedBy,
	})
	if err != nil {
		h.base.fail(w, "POST /motion-thresholds", err, "measuring_point_id", pointID)
		return
	}
	h.base.log.Info("[ENDPOINT/RULE_ENGINE] POST /motion-thresholds - Versión creada",
		"measuring_point_id", pointID, "version", v.Version, "created_by", v.CreatedBy)
	sharedhttp.WriteJSON(w, http.StatusCreated, toMotionVersionResponse(v))
}

func (h *MotionHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.base.pointID(w, r, "POST /motion-thresholds/simulate")
	if !ok {
		return
	}
	var req MotionSimulateRequest
	if err := decode(w, r, &req); err != nil {
		h.base.fail(w, "POST /motion-thresholds/simulate", err, "measuring_point_id", pointID)
		return
	}
	if req.From == nil || req.To == nil {
		h.base.fail(w, "POST /motion-thresholds/simulate", domain.ErrInvalidTimeRange, "measuring_point_id", pointID)
		return
	}
	patch, err := ParseMotionPatch(req.Patch)
	if err != nil {
		h.base.fail(w, "POST /motion-thresholds/simulate", err, "measuring_point_id", pointID)
		return
	}
	result, err := h.simulator.Simulate(r.Context(), services.MotionSimulateCommand{
		PointID: pointID, Patch: patch, From: *req.From, To: *req.To,
	})
	if err != nil {
		h.base.fail(w, "POST /motion-thresholds/simulate", err, "measuring_point_id", pointID)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toMotionSimulationResponse(result))
}

// ParseMotionPatch traduce el patch a un MotionPatch: cada clave es un
// parámetro de §5.2 con su tipo; null y claves desconocidas son ErrInvalidPatch.
// Un patch vacío ({}) es válido: copia la versión vigente.
func ParseMotionPatch(raw map[string]json.RawMessage) (entities.MotionPatch, error) {
	if raw == nil {
		return nil, domain.ErrInvalidPatch
	}
	patch := entities.MotionPatch{}
	for key, value := range raw {
		f, ok := entities.LookupMotionParam(key)
		if !ok || isNull(value) {
			return nil, domain.ErrInvalidPatch
		}
		var err error
		switch f.Kind {
		case entities.MotionParamFloat:
			var v float64
			err = json.Unmarshal(value, &v)
			patch[key] = v
		case entities.MotionParamInt:
			var v int
			err = json.Unmarshal(value, &v)
			patch[key] = v
		case entities.MotionParamBool:
			var v bool
			err = json.Unmarshal(value, &v)
			patch[key] = v
		case entities.MotionParamTempMode:
			var v string
			err = json.Unmarshal(value, &v)
			patch[key] = entities.TempMode(v)
		}
		if err != nil {
			return nil, domain.ErrInvalidPatch
		}
	}
	return patch, nil
}

func motionParamsResponse(p entities.MotionParams) map[string]any {
	out := map[string]any{}
	for _, f := range entities.MotionParamFields() {
		v := f.Get(p)
		if m, ok := v.(entities.TempMode); ok {
			v = string(m)
		}
		out[f.Name] = v
	}
	return out
}

func toMotionVersionResponse(v entities.MotionThresholdVersion) MotionVersionResponse {
	return MotionVersionResponse{
		ID: v.ID, Version: v.Version, PreviousVersionID: v.PreviousVersionID, MeasuringPointID: v.MeasuringPointID,
		Params: motionParamsResponse(v.Params), ValidFrom: v.ValidFrom.UTC(), ValidTo: utcPtr(v.ValidTo),
		Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt.UTC(),
	}
}

func toMotionEpisodes(eps []services.SimulatedMotionEpisode) []SimulatedMotionEpisodeResponse {
	out := make([]SimulatedMotionEpisodeResponse, len(eps))
	for i, e := range eps {
		codes := make([]string, len(e.Reasons))
		for k, c := range e.Reasons {
			codes[k] = string(c)
		}
		out[i] = SimulatedMotionEpisodeResponse{
			OpenedAt: e.OpenedAt.UTC(), ClearedAt: utcPtr(e.ClearedAt), Reasons: codes,
			TempStable: TempStableResponse{From: utcPtr(e.TempStableFrom), To: utcPtr(e.TempStableTo)},
		}
	}
	return out
}

func toMotionSimulationResponse(r services.MotionSimulationResult) MotionSimulationResponse {
	out := MotionSimulationResponse{
		MeasuringPointID: r.PointID, From: r.From, To: r.To,
		Params: motionParamsResponse(r.Proposed), HoursEvaluated: r.HoursEvaluated,
		Episodes: toMotionEpisodes(r.Episodes),
	}
	out.ComparedToCurrent.CurrentVersionID = r.CurrentVersion.ID
	out.ComparedToCurrent.CurrentEpisodes = toMotionEpisodes(r.CurrentEpisodes)
	out.ComparedToCurrent.NewEpisodes = r.NewEpisodes
	out.ComparedToCurrent.RemovedEpisodes = r.RemovedEpisodes
	return out
}
