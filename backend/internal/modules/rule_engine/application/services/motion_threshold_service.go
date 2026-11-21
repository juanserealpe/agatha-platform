package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// CreateMotionThresholdVersionCommand es la entrada de CreateVersion.
type CreateMotionThresholdVersionCommand struct {
	PointID int64
	// Patch: los parámetros no informados se copian de la versión vigente.
	Patch entities.MotionPatch
	// ValidFrom cero ⇒ now. Nunca en el pasado.
	ValidFrom time.Time
	Reason    string
	CreatedBy string
}

// MotionThresholdService implementa los casos de uso de motion_threshold_versions
// con las mismas reglas que FUGA: solo se agregan versiones, nunca
// retroactivas, y la abierta se cierra en la misma transacción que crea la nueva.
type MotionThresholdService struct {
	thresholds repositories.MotionThresholdRepository
	clock      ports.Clock
	log        *logger.Logger
	// source da el largo del historial del punto (aviso
	// WINDOW_LONGER_THAN_HISTORY); nil = no se avisa.
	source ports.RawMeasurementSource
}

func NewMotionThresholdService(thresholds repositories.MotionThresholdRepository, clock ports.Clock, log *logger.Logger) *MotionThresholdService {
	return &MotionThresholdService{thresholds: thresholds, clock: clock, log: log}
}

// WithMeasurementSource activa el aviso WINDOW_LONGER_THAN_HISTORY.
func (s *MotionThresholdService) WithMeasurementSource(source ports.RawMeasurementSource) *MotionThresholdService {
	s.source = source
	return s
}

// historyHours es el largo del historial del punto en horas (-1 si no se sabe).
func (s *MotionThresholdService) historyHours(ctx context.Context, pointID int64, at time.Time) float64 {
	if s.source == nil {
		return -1
	}
	first, err := s.source.FirstMeasurementAt(ctx, pointID)
	if err != nil || first == nil {
		return -1
	}
	return at.Sub(*first).Hours()
}

// CreateVersion crea la versión siguiente del punto. Sin versión previa, el
// parche se aplica sobre la de punto nuevo (modo sombra, SOLO_MOVIMIENTO).
// Un parche sin "rules" copia la tabla de la vigente; con "rules" la
// reemplaza completa. Los avisos de la tabla vuelven en Warnings (se guarda igual).
func (s *MotionThresholdService) CreateVersion(ctx context.Context, cmd CreateMotionThresholdVersionCommand) (entities.MotionThresholdVersion, error) {
	if cmd.PointID <= 0 {
		return entities.MotionThresholdVersion{}, domain.ErrInvalidMeasuringPointID
	}
	if strings.TrimSpace(cmd.Reason) == "" || strings.TrimSpace(cmd.CreatedBy) == "" {
		return entities.MotionThresholdVersion{}, domain.ErrMissingAuthorship
	}
	now := s.clock.Now().UTC()
	validFrom := cmd.ValidFrom.UTC()
	if cmd.ValidFrom.IsZero() {
		validFrom = now
	}
	if validFrom.Before(now) {
		return entities.MotionThresholdVersion{}, domain.ErrValidFromInPast
	}

	var created entities.MotionThresholdVersion
	err := s.thresholds.Transaction(ctx, func(tx repositories.MotionThresholdWriter) error {
		open, err := tx.LockOpen(ctx, cmd.PointID)
		if err != nil {
			return err
		}
		base, version := entities.NewPointMotionParams(), 1
		if open != nil {
			base, version = open.Params, open.Version+1
			if !validFrom.After(open.ValidFrom) {
				return domain.ErrValidFromNotAfterCurrent
			}
		}
		params, err := cmd.Patch.ApplyTo(base)
		if err != nil {
			return err
		}
		params = params.Normalize()
		if err := params.Validate(); err != nil {
			return err
		}
		if open != nil {
			if err := tx.Close(ctx, open.ID, validFrom); err != nil {
				return err
			}
		}
		created = entities.MotionThresholdVersion{
			MeasuringPointID: cmd.PointID,
			Version:          version,
			Params:           params,
			ValidFrom:        validFrom,
			Reason:           strings.TrimSpace(cmd.Reason),
			CreatedBy:        strings.TrimSpace(cmd.CreatedBy),
			CreatedAt:        now,
		}
		if created.ID, err = tx.Insert(ctx, created); err != nil {
			return err
		}
		created.Warnings = entities.MotionRuleWarnings(params.Rules, s.historyHours(ctx, cmd.PointID, validFrom))
		if open != nil {
			previous := open.ID
			created.PreviousVersionID = &previous
		}
		return nil
	})
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return entities.MotionThresholdVersion{}, err
		}
		if errors.Is(err, repositories.ErrConflict) {
			s.log.Warn("[MotionThresholdService] Versión de movimiento concurrente: se rechaza para reintentar",
				"measuring_point_id", cmd.PointID, "error", err)
			return entities.MotionThresholdVersion{}, domain.ErrThresholdConflict
		}
		s.log.Error("[MotionThresholdService] Error guardando la versión de movimiento",
			"measuring_point_id", cmd.PointID, "error", err)
		return entities.MotionThresholdVersion{}, domain.ErrPersistingThresholds
	}

	s.log.Info("[MotionThresholdService] Versión de movimiento creada",
		"measuring_point_id", created.MeasuringPointID, "version", created.Version,
		"alerts_enabled", created.Params.AlertsEnabled, "temp_mode", created.Params.TempMode,
		"valid_from", created.ValidFrom, "reason", created.Reason, "created_by", created.CreatedBy,
		"rules", len(created.Params.Rules), "warnings", created.Warnings)
	return created, nil
}

// EnsureInitialVersion crea la v1 de punto nuevo (valid_from = now) de un
// punto sin versión. created = false ⇒ ya tenía.
func (s *MotionThresholdService) EnsureInitialVersion(ctx context.Context, pointID int64) (bool, error) {
	created := false
	now := s.clock.Now().UTC()
	err := s.thresholds.Transaction(ctx, func(tx repositories.MotionThresholdWriter) error {
		open, err := tx.LockOpen(ctx, pointID)
		if err != nil || open != nil {
			return err
		}
		_, err = tx.Insert(ctx, entities.MotionThresholdVersion{
			MeasuringPointID: pointID,
			Version:          1,
			Params:           entities.NewPointMotionParams(),
			ValidFrom:        now,
			Reason:           entities.FirstUseSeedReason,
			CreatedBy:        entities.SystemCreatedBy,
			CreatedAt:        now,
		})
		created = err == nil
		return err
	})
	if err != nil {
		if errors.Is(err, repositories.ErrConflict) {
			return false, nil // otro proceso la está creando
		}
		s.log.Error("[MotionThresholdService] Error creando la versión inicial de movimiento",
			"measuring_point_id", pointID, "error", err)
		return false, domain.ErrPersistingThresholds
	}
	if created {
		s.log.Info("[MotionThresholdService] Punto sin umbrales de movimiento: v1 de punto nuevo (modo sombra, SOLO_MOVIMIENTO)",
			"measuring_point_id", pointID, "valid_from", now)
	}
	return created, nil
}

// GetCurrent devuelve la versión vigente ahora.
func (s *MotionThresholdService) GetCurrent(ctx context.Context, pointID int64) (entities.MotionThresholdVersion, error) {
	if pointID <= 0 {
		return entities.MotionThresholdVersion{}, domain.ErrInvalidMeasuringPointID
	}
	v, err := s.thresholds.GetEffectiveAt(ctx, pointID, s.clock.Now().UTC())
	if err != nil {
		s.log.Error("[MotionThresholdService] Error consultando la versión vigente", "measuring_point_id", pointID, "error", err)
		return entities.MotionThresholdVersion{}, domain.ErrConsultingThresholds
	}
	if v == nil {
		return entities.MotionThresholdVersion{}, domain.ErrMotionThresholdsNotFound
	}
	if v.Version > 1 {
		history, err := s.GetHistory(ctx, pointID)
		if err != nil {
			return entities.MotionThresholdVersion{}, err
		}
		for _, h := range history {
			if h.ID == v.ID {
				v.PreviousVersionID = h.PreviousVersionID
			}
		}
	}
	return *v, nil
}

// GetHistory devuelve todas las versiones del punto, ascendentes.
func (s *MotionThresholdService) GetHistory(ctx context.Context, pointID int64) ([]entities.MotionThresholdVersion, error) {
	if pointID <= 0 {
		return nil, domain.ErrInvalidMeasuringPointID
	}
	history, err := s.thresholds.GetHistory(ctx, pointID)
	if err != nil {
		s.log.Error("[MotionThresholdService] Error consultando el historial", "measuring_point_id", pointID, "error", err)
		return nil, domain.ErrConsultingThresholds
	}
	for n := 1; n < len(history); n++ {
		previous := history[n-1].ID
		history[n].PreviousVersionID = &previous
	}
	return history, nil
}

// MotionTimelineSegment es un tramo de vigencia recortado a [from, to).
type MotionTimelineSegment struct {
	From, To time.Time
	Version  entities.MotionThresholdVersion
}

// GetTimeline devuelve los tramos de vigencia de las versiones de movimiento
// que se solapan con [from, to), recortados al rango (como en FUGA).
func (s *MotionThresholdService) GetTimeline(ctx context.Context, pointID int64, from, to time.Time) ([]MotionTimelineSegment, error) {
	if !from.Before(to) {
		return nil, domain.ErrInvalidTimeRange
	}
	history, err := s.GetHistory(ctx, pointID)
	if err != nil {
		return nil, err
	}
	var out []MotionTimelineSegment
	for _, v := range history {
		start, end := v.ValidFrom, to
		if v.ValidTo != nil && v.ValidTo.Before(end) {
			end = *v.ValidTo
		}
		if start.Before(from) {
			start = from
		}
		if start.Before(end) {
			out = append(out, MotionTimelineSegment{From: start.UTC(), To: end.UTC(), Version: v})
		}
	}
	return out, nil
}
