package http

import (
	"bytes"
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

// MotionThresholdUseCases son los casos de uso de motion_threshold_versions
// (en producción, *services.MotionThresholdService).
type MotionThresholdUseCases interface {
	GetCurrent(ctx context.Context, pointID int64) (entities.MotionThresholdVersion, error)
	GetHistory(ctx context.Context, pointID int64) ([]entities.MotionThresholdVersion, error)
	CreateVersion(ctx context.Context, cmd services.CreateMotionThresholdVersionCommand) (entities.MotionThresholdVersion, error)
	GetTimeline(ctx context.Context, pointID int64, from, to time.Time) ([]services.MotionTimelineSegment, error)
}

// MotionSimulator es la simulación de movimiento (*services.MotionSimulationService).
type MotionSimulator interface {
	Simulate(ctx context.Context, cmd services.MotionSimulateCommand) (services.MotionSimulationResult, error)
}

// MotionObserver es GET …/observed (*services.MotionObservedService).
type MotionObserver interface {
	Observed(ctx context.Context, pointID int64, days int) (services.MotionObservedResult, error)
}

// Rutas de movimiento (§6.4).
const (
	RouteMotionCurrent  = "GET /api/rule-engine/points/{id}/motion-thresholds"
	RouteMotionHistory  = "GET /api/rule-engine/points/{id}/motion-thresholds/history"
	RouteMotionTimeline = "GET /api/rule-engine/points/{id}/motion-thresholds/timeline"
	RouteMotionCreate   = "POST /api/rule-engine/points/{id}/motion-thresholds"
	RouteMotionSimulate = "POST /api/rule-engine/points/{id}/motion-thresholds/simulate"
	RouteMotionObserved = "GET /api/rule-engine/points/{id}/motion-thresholds/observed"
)

// MotionHandler expone las versiones de movimiento y su simulación. Reusa la
// validación, el decode y el manejo de errores del ThresholdHandler.
type MotionHandler struct {
	thresholds MotionThresholdUseCases
	simulator  MotionSimulator
	observer   MotionObserver // nil ⇒ la ruta observed no se registra
	base       *ThresholdHandler
}

// WithObserver activa GET …/motion-thresholds/observed.
func (h *MotionHandler) WithObserver(o MotionObserver) *MotionHandler {
	h.observer = o
	return h
}

func NewMotionHandler(thresholds MotionThresholdUseCases, simulator MotionSimulator, log *logger.Logger) *MotionHandler {
	return &MotionHandler{thresholds: thresholds, simulator: simulator, base: &ThresholdHandler{log: log}}
}

func (h *MotionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteMotionCurrent, h.GetCurrent)
	mux.HandleFunc(RouteMotionHistory, h.GetHistory)
	mux.HandleFunc(RouteMotionTimeline, h.GetTimeline)
	mux.HandleFunc(RouteMotionCreate, h.Create)
	mux.HandleFunc(RouteMotionSimulate, h.Simulate)
	if h.observer != nil {
		mux.HandleFunc(RouteMotionObserved, h.Observed)
	}
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
// parámetros de §5.2 por nombre (sin los retirados abrupt_deg y
// abrupt_hours) y la tabla de lo normal en params.rules. Warnings solo en la
// respuesta del POST.
type MotionVersionResponse struct {
	ID                int64          `json:"id"`
	Version           int            `json:"version"`
	PreviousVersionID *int64         `json:"previous_version_id"`
	MeasuringPointID  int64          `json:"measuring_point_id"`
	Params            map[string]any `json:"params"`
	Warnings          []string       `json:"warnings,omitempty"`
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

// MotionTimelineSegmentResponse es un tramo de vigencia recortado.
type MotionTimelineSegmentResponse struct {
	From    time.Time             `json:"from"`
	To      time.Time             `json:"to"`
	Version MotionVersionResponse `json:"version"`
}

// SimulatedMotionEpisodeResponse es un episodio de la simulación.
// TableLevel es el nivel calculado máximo; SentLevel, el enviado.
type SimulatedMotionEpisodeResponse struct {
	OpenedAt    time.Time          `json:"opened_at"`
	ClearedAt   *time.Time         `json:"cleared_at"`
	EpisodeKind string             `json:"episode_kind"`
	TableLevel  string             `json:"table_level"`
	SentLevel   string             `json:"sent_level"`
	Reasons     []string           `json:"reasons"`
	TempStable  TempStableResponse `json:"temp_stable"`
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

// GetTimeline devuelve los tramos de vigencia recortados a [from, to) (RFC3339, obligatorios).
func (h *MotionHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.base.pointID(w, r, "GET /motion-thresholds/timeline")
	if !ok {
		return
	}
	query := r.URL.Query()
	from, errFrom := time.Parse(time.RFC3339, query.Get("from"))
	to, errTo := time.Parse(time.RFC3339, query.Get("to"))
	if errFrom != nil || errTo != nil {
		h.base.fail(w, "GET /motion-thresholds/timeline", domain.ErrInvalidTimeRange, "from", query.Get("from"), "to", query.Get("to"))
		return
	}
	segments, err := h.thresholds.GetTimeline(r.Context(), pointID, from.UTC(), to.UTC())
	if err != nil {
		h.base.fail(w, "GET /motion-thresholds/timeline", err, "measuring_point_id", pointID)
		return
	}
	out := make([]MotionTimelineSegmentResponse, len(segments))
	for i, s := range segments {
		out[i] = MotionTimelineSegmentResponse{From: s.From, To: s.To, Version: toMotionVersionResponse(s.Version)}
	}
	sharedhttp.WriteJSON(w, http.StatusOK, out)
}

// MotionObservedRowResponse es lo observado para una fila y un eje.
type MotionObservedRowResponse struct {
	WindowHours int      `json:"window_hours"`
	RuleAxis    string   `json:"rule_axis"`
	Axis        string   `json:"axis"`
	AmarilloDeg float64  `json:"amarillo_deg"`
	Samples     int      `json:"samples"`
	P95         *float64 `json:"p95"`
	P99         *float64 `json:"p99"`
	Max         *float64 `json:"max"`
}

// MotionObservedResponse es GET …/observed.
type MotionObservedResponse struct {
	MeasuringPointID int64                       `json:"measuring_point_id"`
	Days             int                         `json:"days"`
	From             *time.Time                  `json:"from"`
	To               *time.Time                  `json:"to"`
	VersionID        int64                       `json:"threshold_version_id"`
	Rows             []MotionObservedRowResponse `json:"rows"`
}

// Observed (P2, solo lectura): p95, p99 y máximo del cambio de cada fila de la
// tabla vigente en los últimos ?days días (90 por defecto), fuera de episodios.
func (h *MotionHandler) Observed(w http.ResponseWriter, r *http.Request) {
	pointID, ok := h.base.pointID(w, r, "GET /motion-thresholds/observed")
	if !ok {
		return
	}
	days := services.DefaultObservedDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			h.base.fail(w, "GET /motion-thresholds/observed", domain.ErrMotionObservedDays, "days", raw)
			return
		}
		days = n
	}
	res, err := h.observer.Observed(r.Context(), pointID, days)
	if err != nil {
		h.base.fail(w, "GET /motion-thresholds/observed", err, "measuring_point_id", pointID)
		return
	}
	out := MotionObservedResponse{MeasuringPointID: res.PointID, Days: res.Days, VersionID: res.VersionID, Rows: []MotionObservedRowResponse{}}
	if !res.To.IsZero() {
		out.From, out.To = &res.From, &res.To
	}
	for _, row := range res.Rows {
		out.Rows = append(out.Rows, MotionObservedRowResponse{WindowHours: row.WindowHours, RuleAxis: row.RuleAxis, Axis: row.Axis,
			AmarilloDeg: row.AmarilloDeg, Samples: row.Samples, P95: row.P95, P99: row.P99, Max: row.Max})
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

// MotionRuleRequest es una fila de patch.rules. window_hours, axis y
// amarillo_deg son obligatorios; naranja_deg y rojo_deg pueden ser null.
type MotionRuleRequest struct {
	WindowHours *int     `json:"window_hours"`
	Axis        *string  `json:"axis"`
	AmarilloDeg *float64 `json:"amarillo_deg"`
	NaranjaDeg  *float64 `json:"naranja_deg"`
	RojoDeg     *float64 `json:"rojo_deg"`
}

// parseMotionRules traduce patch.rules; un campo obligatorio ausente o una
// clave desconocida es ErrInvalidPatch.
func parseMotionRules(value json.RawMessage) ([]entities.MotionRule, error) {
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.DisallowUnknownFields()
	var rows []MotionRuleRequest
	if err := dec.Decode(&rows); err != nil {
		return nil, domain.ErrInvalidPatch
	}
	out := make([]entities.MotionRule, len(rows))
	for i, r := range rows {
		if r.WindowHours == nil || r.Axis == nil || r.AmarilloDeg == nil {
			return nil, domain.ErrInvalidPatch
		}
		out[i] = entities.MotionRule{WindowHours: *r.WindowHours, Axis: *r.Axis, AmarilloDeg: *r.AmarilloDeg,
			NaranjaDeg: r.NaranjaDeg, RojoDeg: r.RojoDeg}
	}
	return out, nil
}

// ParseMotionPatch traduce el patch a un MotionPatch: cada clave es un
// parámetro de §5.2 con su tipo, o "rules" (la tabla completa); null y
// claves desconocidas son ErrInvalidPatch; abrupt_deg y abrupt_hours,
// ErrMotionAbruptRetired. Un patch vacío ({}) es válido: copia la versión vigente.
func ParseMotionPatch(raw map[string]json.RawMessage) (entities.MotionPatch, error) {
	if raw == nil {
		return nil, domain.ErrInvalidPatch
	}
	patch := entities.MotionPatch{}
	for key, value := range raw {
		if key == entities.MotionRulesParam {
			rules, err := parseMotionRules(value)
			if err != nil {
				return nil, err
			}
			patch[key] = rules
			continue
		}
		f, ok := entities.LookupMotionParam(key)
		if ok && f.Retired {
			return nil, domain.ErrMotionAbruptRetired
		}
		if !ok || isNull(value) {
			return nil, domain.ErrInvalidPatch
		}
		var err error
		switch f.Kind {
		case entities.MotionParamFraction:
			var v float64
			err = json.Unmarshal(value, &v)
			patch[key] = v
		case entities.MotionParamSendMode, entities.MotionParamLevel:
			var v string
			err = json.Unmarshal(value, &v)
			patch[key] = v
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
		if !f.Retired {
			out[f.Name] = f.GetString(p)
		}
	}
	rules := entities.CloneMotionRules(p.Rules)
	if rules == nil {
		rules = []entities.MotionRule{}
	}
	out[entities.MotionRulesParam] = rules
	return out
}

func toMotionVersionResponse(v entities.MotionThresholdVersion) MotionVersionResponse {
	return MotionVersionResponse{
		ID: v.ID, Version: v.Version, PreviousVersionID: v.PreviousVersionID, MeasuringPointID: v.MeasuringPointID,
		Params: motionParamsResponse(v.Params), Warnings: v.Warnings, ValidFrom: v.ValidFrom.UTC(), ValidTo: utcPtr(v.ValidTo),
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
			EpisodeKind: string(e.Kind), TableLevel: string(e.TableLevel), SentLevel: string(e.SentLevel),
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
