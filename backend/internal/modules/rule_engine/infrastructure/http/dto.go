// Package http expone los umbrales del Rule Engine bajo /api/rule-engine:
// consulta, creación de versiones (individual y masiva) y simulación.
package http

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// CreateVersionRequest es el cuerpo de POST /points/{id}/thresholds.
//
// CreatedBy es TEMPORAL: mientras no haya autenticación de usuarios, el autor
// viaja en el cuerpo. Cuando exista, se tomará del token y este campo se elimina.
type CreateVersionRequest struct {
	EventType string                     `json:"event_type"`
	Patch     map[string]json.RawMessage `json:"patch"`
	ValidFrom *time.Time                 `json:"valid_from"`
	Reason    string                     `json:"reason"`
	CreatedBy string                     `json:"created_by"`
}

// BulkCreateVersionRequest es el cuerpo de POST /thresholds/bulk.
type BulkCreateVersionRequest struct {
	PointIDs []int64 `json:"point_ids"`
	CreateVersionRequest
}

// SimulateRequest es el cuerpo de POST /points/{id}/thresholds/simulate.
type SimulateRequest struct {
	EventType string                     `json:"event_type"`
	Patch     map[string]json.RawMessage `json:"patch"`
	From      *time.Time                 `json:"from"`
	To        *time.Time                 `json:"to"`
}

// LevelCutsResponse son tres valores de I, uno por nivel.
type LevelCutsResponse struct {
	Amarillo float64 `json:"amarillo"`
	Naranja  float64 `json:"naranja"`
	Rojo     float64 `json:"rojo"`
}

// ThresholdParamsResponse son los parámetros de una versión (o de una simulación).
type ThresholdParamsResponse struct {
	Amarillo                   float64           `json:"amarillo"`
	Naranja                    float64           `json:"naranja"`
	Rojo                       float64           `json:"rojo"`
	MinHours                   int               `json:"min_hours"`
	EscalateAmarilloAfterHours *int              `json:"escalate_amarillo_after_hours"`
	EscalateNaranjaAfterHours  *int              `json:"escalate_naranja_after_hours"`
	ExitMargin                 float64           `json:"exit_margin"`
	ExitHours                  int               `json:"exit_hours"`
	Exit                       LevelCutsResponse `json:"exit"`
	ConfirmNaranjaWithPlanitud bool              `json:"confirm_naranja_with_planitud"`
	DPlMin                     *float64          `json:"d_pl_min"`
	PersistenceMode            string            `json:"persistence_mode"`
	PersistenceWindowHours     *int              `json:"persistence_window_hours"`
	PersistenceMinHours        *int              `json:"persistence_min_hours"`
}

// ThresholdVersionResponse es una versión de umbrales.
type ThresholdVersionResponse struct {
	ID                int64  `json:"id"`
	Version           int    `json:"version"`
	PreviousVersionID *int64 `json:"previous_version_id"`
	MeasuringPointID  int64  `json:"measuring_point_id"`
	EventType         string `json:"event_type"`
	ThresholdParamsResponse
	ValidFrom time.Time  `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to"`
	Reason    string     `json:"reason"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
}

// TimelineSegmentResponse es un tramo de vigencia recortado al rango pedido.
type TimelineSegmentResponse struct {
	From    time.Time                `json:"from"`
	To      time.Time                `json:"to"`
	Version ThresholdVersionResponse `json:"version"`
}

// SimulatedEpisodeResponse es un episodio de la simulación (horas en UTC).
type SimulatedEpisodeResponse struct {
	OpenedAt       time.Time  `json:"opened_at"`
	ClearedAt      *time.Time `json:"cleared_at"`
	MaxLevel       string     `json:"max_level"`
	FirstNaranjaAt *time.Time `json:"first_naranja_at"`
	FirstRojoAt    *time.Time `json:"first_rojo_at"`
	Hours          int        `json:"hours"`
}

// SimulationSummaryResponse resume los episodios propuestos.
type SimulationSummaryResponse struct {
	Episodes   int            `json:"episodes"`
	ByMaxLevel map[string]int `json:"by_max_level"`
}

// ComparedToCurrentResponse compara con los parámetros vigentes en el mismo rango.
type ComparedToCurrentResponse struct {
	CurrentVersionID int64 `json:"current_version_id"`
	CurrentEpisodes  int   `json:"current_episodes"`
	NewEpisodes      int   `json:"new_episodes"`
	RemovedEpisodes  int   `json:"removed_episodes"`
}

// SimulationResponse es la respuesta de simulate.
type SimulationResponse struct {
	MeasuringPointID  int64                      `json:"measuring_point_id"`
	EventType         string                     `json:"event_type"`
	From              time.Time                  `json:"from"`
	To                time.Time                  `json:"to"`
	Thresholds        ThresholdParamsResponse    `json:"thresholds"`
	HoursEvaluated    int                        `json:"hours_evaluated"`
	Episodes          []SimulatedEpisodeResponse `json:"episodes"`
	Summary           SimulationSummaryResponse  `json:"summary"`
	ComparedToCurrent ComparedToCurrentResponse  `json:"compared_to_current"`
}

func toParamsResponse(p entities.ThresholdParams) ThresholdParamsResponse {
	return ThresholdParamsResponse{
		Amarillo:                   p.Amarillo,
		Naranja:                    p.Naranja,
		Rojo:                       p.Rojo,
		MinHours:                   p.MinHours,
		EscalateAmarilloAfterHours: p.EscalateAmarilloAfterHours,
		EscalateNaranjaAfterHours:  p.EscalateNaranjaAfterHours,
		ExitMargin:                 p.ExitMargin,
		ExitHours:                  p.ExitHours,
		Exit: LevelCutsResponse{
			Amarillo: p.ExitFor(entities.LevelAmarillo),
			Naranja:  p.ExitFor(entities.LevelNaranja),
			Rojo:     p.ExitFor(entities.LevelRojo),
		},
		ConfirmNaranjaWithPlanitud: p.ConfirmNaranjaWithPlanitud,
		DPlMin:                     p.DPlMin,
		PersistenceMode:            string(p.Normalize().PersistenceMode),
		PersistenceWindowHours:     p.PersistenceWindowHours,
		PersistenceMinHours:        p.PersistenceMinHours,
	}
}

func toVersionResponse(v entities.ThresholdVersion) ThresholdVersionResponse {
	return ThresholdVersionResponse{
		ID:                      v.ID,
		Version:                 v.Version,
		PreviousVersionID:       v.PreviousVersionID,
		MeasuringPointID:        v.MeasuringPointID,
		EventType:               string(v.EventType),
		ThresholdParamsResponse: toParamsResponse(v.ThresholdParams),
		ValidFrom:               v.ValidFrom.UTC(),
		ValidTo:                 utcPtr(v.ValidTo),
		Reason:                  v.Reason,
		CreatedBy:               v.CreatedBy,
		CreatedAt:               v.CreatedAt.UTC(),
	}
}

func toVersionsResponse(versions []entities.ThresholdVersion) []ThresholdVersionResponse {
	out := make([]ThresholdVersionResponse, len(versions))
	for n, v := range versions {
		out[n] = toVersionResponse(v)
	}
	return out
}

func toTimelineResponse(segments []entities.ThresholdSegment) []TimelineSegmentResponse {
	out := make([]TimelineSegmentResponse, len(segments))
	for n, s := range segments {
		out[n] = TimelineSegmentResponse{From: s.From.UTC(), To: s.To.UTC(), Version: toVersionResponse(s.Version)}
	}
	return out
}

func toSimulationResponse(r services.SimulationResult) SimulationResponse {
	episodes := make([]SimulatedEpisodeResponse, len(r.Episodes))
	for n, e := range r.Episodes {
		episodes[n] = SimulatedEpisodeResponse{
			OpenedAt:       e.OpenedAt.UTC(),
			ClearedAt:      utcPtr(e.ClearedAt),
			MaxLevel:       string(e.MaxLevel),
			FirstNaranjaAt: utcPtr(e.FirstNaranjaAt),
			FirstRojoAt:    utcPtr(e.FirstRojoAt),
			Hours:          e.Hours,
		}
	}

	byMaxLevel := map[string]int{
		string(entities.LevelAmarillo): r.ByMaxLevel[entities.LevelAmarillo],
		string(entities.LevelNaranja):  r.ByMaxLevel[entities.LevelNaranja],
		string(entities.LevelRojo):     r.ByMaxLevel[entities.LevelRojo],
	}

	return SimulationResponse{
		MeasuringPointID: r.PointID,
		EventType:        string(r.EventType),
		From:             r.From,
		To:               r.To,
		Thresholds:       toParamsResponse(r.Proposed),
		HoursEvaluated:   r.HoursEvaluated,
		Episodes:         episodes,
		Summary:          SimulationSummaryResponse{Episodes: len(r.Episodes), ByMaxLevel: byMaxLevel},
		ComparedToCurrent: ComparedToCurrentResponse{
			CurrentVersionID: r.CurrentVersion.ID,
			CurrentEpisodes:  len(r.CurrentEpisodes),
			NewEpisodes:      r.NewEpisodes,
			RemovedEpisodes:  r.RemovedEpisodes,
		},
	}
}

// ParsePatch traduce el objeto patch a un ThresholdPatch. Un campo ausente se
// copia de la versión vigente; null solo se acepta en los campos anulables
// (escalamientos, d_pl_min y horas de ventana) y los deja en NULL.
func ParsePatch(raw map[string]json.RawMessage) (entities.ThresholdPatch, error) {
	var patch entities.ThresholdPatch
	if raw == nil {
		return patch, domain.ErrInvalidPatch
	}

	for key, value := range raw {
		var err error
		switch key {
		case "amarillo":
			patch.Amarillo, err = required[float64](value)
		case "naranja":
			patch.Naranja, err = required[float64](value)
		case "rojo":
			patch.Rojo, err = required[float64](value)
		case "exit_margin":
			patch.ExitMargin, err = required[float64](value)
		case "min_hours":
			patch.MinHours, err = required[int](value)
		case "exit_hours":
			patch.ExitHours, err = required[int](value)
		case "confirm_naranja_with_planitud":
			patch.ConfirmNaranjaWithPlanitud, err = required[bool](value)
		case "persistence_mode":
			var mode entities.Optional[string]
			mode, err = required[string](value)
			patch.PersistenceMode = entities.Some(entities.PersistenceMode(mode.Value))
		case "escalate_amarillo_after_hours":
			patch.EscalateAmarilloAfterHours, err = nullable[int](value)
		case "escalate_naranja_after_hours":
			patch.EscalateNaranjaAfterHours, err = nullable[int](value)
		case "persistence_window_hours":
			patch.PersistenceWindowHours, err = nullable[int](value)
		case "persistence_min_hours":
			patch.PersistenceMinHours, err = nullable[int](value)
		case "d_pl_min":
			patch.DPlMin, err = nullable[float64](value)
		default:
			return entities.ThresholdPatch{}, domain.ErrInvalidPatch
		}
		if err != nil {
			return entities.ThresholdPatch{}, domain.ErrInvalidPatch
		}
	}
	return patch, nil
}

func isNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func required[T any](value json.RawMessage) (entities.Optional[T], error) {
	var v T
	if isNull(value) {
		return entities.Optional[T]{}, domain.ErrInvalidPatch
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return entities.Optional[T]{}, err
	}
	return entities.Some(v), nil
}

func nullable[T any](value json.RawMessage) (entities.Optional[*T], error) {
	if isNull(value) {
		return entities.Some[*T](nil), nil
	}
	var v T
	if err := json.Unmarshal(value, &v); err != nil {
		return entities.Optional[*T]{}, err
	}
	return entities.Some(&v), nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
