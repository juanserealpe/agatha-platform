package services

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/services"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// ThresholdService implementa los casos de uso de umbrales versionados (los
// expone infrastructure/http bajo /api/rule-engine).
//
// Reglas: nunca UPDATE ni DELETE de una versión, salvo cerrar la abierta
// (valid_to = valid_from de la nueva) en la MISMA transacción que crea la
// nueva; nunca retroactivo (valid_from >= now).
type ThresholdService struct {
	thresholds repositories.ThresholdRepository
	clock      ports.Clock
	log        *logger.Logger
}

func NewThresholdService(thresholds repositories.ThresholdRepository, clock ports.Clock, log *logger.Logger) *ThresholdService {
	return &ThresholdService{thresholds: thresholds, clock: clock, log: log}
}

// CreateThresholdVersion crea una versión nueva para un punto. Los campos que
// el parche no informa se copian de la versión abierta (o de la plantilla si
// el punto no tiene ninguna). ValidFrom cero ⇒ now; en el pasado ⇒ error.
func (s *ThresholdService) CreateThresholdVersion(ctx context.Context, cmd CreateThresholdVersionCommand) (entities.ThresholdVersion, error) {
	created, err := s.CreateThresholdVersionBulk(ctx, CreateThresholdVersionBulkCommand{
		PointIDs:  []int64{cmd.PointID},
		EventType: cmd.EventType,
		Patch:     cmd.Patch,
		ValidFrom: cmd.ValidFrom,
		Reason:    cmd.Reason,
		CreatedBy: cmd.CreatedBy,
	})
	if err != nil {
		return entities.ThresholdVersion{}, err
	}
	return created[0], nil
}

// CreateThresholdVersionBulk aplica el mismo parche a varios puntos en UNA
// transacción: si un punto falla la validación no se escribe ninguno. Sin
// PointIDs aplica a todos los puntos que ya tienen alguna versión.
func (s *ThresholdService) CreateThresholdVersionBulk(ctx context.Context, cmd CreateThresholdVersionBulkCommand) ([]entities.ThresholdVersion, error) {
	eventType, validFrom, err := s.validateCommand(cmd)
	if err != nil {
		return nil, err
	}

	points := slices.Clone(cmd.PointIDs)
	if len(points) == 0 {
		if points, err = s.thresholds.ListPoints(ctx, eventType); err != nil {
			s.log.Error("[ThresholdService] Error listando los puntos con umbrales", "error", err)
			return nil, domain.ErrConsultingThresholds
		}
	}
	if len(points) == 0 {
		return nil, domain.ErrNoPoints
	}
	slices.Sort(points)
	points = slices.Compact(points)
	if points[0] <= 0 {
		return nil, domain.ErrInvalidMeasuringPointID
	}

	created := make([]entities.ThresholdVersion, 0, len(points))
	createdAt := s.clock.Now().UTC()

	err = s.thresholds.Transaction(ctx, func(tx repositories.ThresholdWriter) error {
		for _, pointID := range points {
			v, err := supersede(ctx, tx, pointID, eventType, cmd.Patch, validFrom, createdAt, cmd.Reason, cmd.CreatedBy)
			if err != nil {
				return err
			}
			created = append(created, v)
		}
		return nil
	})
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		if errors.Is(err, repositories.ErrConflict) {
			s.log.Warn("[ThresholdService] Versión de umbrales concurrente: se rechaza para reintentar", "points", points, "error", err)
			return nil, domain.ErrThresholdConflict
		}
		s.log.Error("[ThresholdService] Error guardando la versión de umbrales", "points", points, "error", err)
		return nil, domain.ErrPersistingThresholds
	}

	for _, v := range created {
		s.log.Info("[ThresholdService] Versión de umbrales creada",
			"measuring_point_id", v.MeasuringPointID, "event_type", v.EventType, "version", v.Version,
			"amarillo", v.Amarillo, "naranja", v.Naranja, "rojo", v.Rojo,
			"persistence_mode", v.PersistenceMode,
			"valid_from", v.ValidFrom, "reason", v.Reason, "created_by", v.CreatedBy)
	}

	return created, nil
}

// EnsureInitialVersion crea la versión 1 (plantilla, valid_from = now) de un
// punto que no tiene ninguna. Es el "primer uso" de un punto que no existía al
// correr el seed. created = false ⇒ ya tenía versión.
func (s *ThresholdService) EnsureInitialVersion(ctx context.Context, pointID int64, eventType entities.EventType) (bool, error) {
	created := false
	now := s.clock.Now().UTC()

	err := s.thresholds.Transaction(ctx, func(tx repositories.ThresholdWriter) error {
		open, err := tx.LockOpen(ctx, pointID, eventType)
		if err != nil || open != nil {
			return err
		}
		_, err = tx.Insert(ctx, entities.ThresholdVersion{
			MeasuringPointID: pointID,
			EventType:        eventType,
			Version:          1,
			ThresholdParams:  entities.DefaultThresholdParams(),
			ValidFrom:        now,
			Reason:           entities.FirstUseSeedReason,
			CreatedBy:        entities.SystemCreatedBy,
			CreatedAt:        now,
		})
		created = err == nil
		return err
	})
	if err != nil {
		s.log.Error("[ThresholdService] Error creando la versión inicial de umbrales",
			"measuring_point_id", pointID, "error", err)
		return false, domain.ErrPersistingThresholds
	}

	if created {
		s.log.Info("[ThresholdService] Punto sin umbrales: versión 1 creada con la plantilla (primer uso)",
			"measuring_point_id", pointID, "event_type", eventType, "valid_from", now)
	}
	return created, nil
}

// GetCurrentThresholds devuelve la versión vigente ahora.
func (s *ThresholdService) GetCurrentThresholds(ctx context.Context, pointID int64, eventType entities.EventType) (entities.ThresholdVersion, error) {
	if pointID <= 0 {
		return entities.ThresholdVersion{}, domain.ErrInvalidMeasuringPointID
	}

	v, err := s.thresholds.GetEffectiveAt(ctx, pointID, normalizeEventType(eventType), s.clock.Now().UTC())
	if err != nil {
		s.log.Error("[ThresholdService] Error consultando los umbrales vigentes", "measuring_point_id", pointID, "error", err)
		return entities.ThresholdVersion{}, domain.ErrConsultingThresholds
	}
	if v == nil {
		return entities.ThresholdVersion{}, domain.ErrThresholdsNotFound
	}
	if v.Version > 1 {
		history, err := s.thresholds.GetHistory(ctx, pointID, normalizeEventType(eventType))
		if err != nil {
			s.log.Error("[ThresholdService] Error consultando el historial de umbrales", "measuring_point_id", pointID, "error", err)
			return entities.ThresholdVersion{}, domain.ErrConsultingThresholds
		}
		for _, h := range withPreviousIDs(history) {
			if h.ID == v.ID {
				v.PreviousVersionID = h.PreviousVersionID
			}
		}
	}
	return *v, nil
}

// GetThresholdHistory devuelve todas las versiones del punto (ascendentes).
func (s *ThresholdService) GetThresholdHistory(ctx context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error) {
	if pointID <= 0 {
		return nil, domain.ErrInvalidMeasuringPointID
	}

	history, err := s.thresholds.GetHistory(ctx, pointID, normalizeEventType(eventType))
	if err != nil {
		s.log.Error("[ThresholdService] Error consultando el historial de umbrales", "measuring_point_id", pointID, "error", err)
		return nil, domain.ErrConsultingThresholds
	}
	return withPreviousIDs(history), nil
}

// GetThresholdTimeline devuelve los tramos de umbrales que se solapan con
// [from, to), recortados a los extremos del rango (para graficar).
func (s *ThresholdService) GetThresholdTimeline(ctx context.Context, pointID int64, eventType entities.EventType, from, to time.Time,
) ([]entities.ThresholdSegment, error) {
	if pointID <= 0 {
		return nil, domain.ErrInvalidMeasuringPointID
	}
	if !from.Before(to) {
		return nil, domain.ErrInvalidTimeRange
	}

	versions, err := s.thresholds.GetOverlapping(ctx, pointID, normalizeEventType(eventType), from, to)
	if err != nil {
		s.log.Error("[ThresholdService] Error consultando la línea de tiempo de umbrales",
			"measuring_point_id", pointID, "from", from, "to", to, "error", err)
		return nil, domain.ErrConsultingThresholds
	}
	return domainServices.ClipTimeline(versions, from, to), nil
}

// validateCommand valida lo que no depende de la versión vigente.
func (s *ThresholdService) validateCommand(cmd CreateThresholdVersionBulkCommand) (entities.EventType, time.Time, error) {
	eventType := normalizeEventType(cmd.EventType)
	if eventType != entities.EventTypeLeak {
		return "", time.Time{}, domain.ErrInvalidEventType
	}
	if strings.TrimSpace(cmd.Reason) == "" || strings.TrimSpace(cmd.CreatedBy) == "" {
		return "", time.Time{}, domain.ErrMissingAuthorship
	}

	now := s.clock.Now().UTC()
	validFrom := cmd.ValidFrom.UTC()
	if cmd.ValidFrom.IsZero() {
		validFrom = now
	}
	if validFrom.Before(now) {
		return "", time.Time{}, domain.ErrValidFromInPast
	}

	return eventType, validFrom, nil
}

// supersede cierra la versión abierta del punto e inserta la nueva, dentro de
// la transacción recibida.
func supersede(
	ctx context.Context,
	tx repositories.ThresholdWriter,
	pointID int64,
	eventType entities.EventType,
	patch entities.ThresholdPatch,
	validFrom, createdAt time.Time,
	reason, createdBy string,
) (entities.ThresholdVersion, error) {
	open, err := tx.LockOpen(ctx, pointID, eventType)
	if err != nil {
		return entities.ThresholdVersion{}, err
	}

	base, version := entities.DefaultThresholdParams(), 1
	if open != nil {
		base, version = open.ThresholdParams, open.Version+1
		if !validFrom.After(open.ValidFrom) {
			return entities.ThresholdVersion{}, domain.ErrValidFromNotAfterCurrent
		}
	}

	params := patch.ApplyTo(base).Normalize()
	if err := params.Validate(); err != nil {
		return entities.ThresholdVersion{}, err
	}

	if open != nil {
		if err := tx.Close(ctx, open.ID, validFrom); err != nil {
			return entities.ThresholdVersion{}, err
		}
	}

	v := entities.ThresholdVersion{
		MeasuringPointID: pointID,
		EventType:        eventType,
		Version:          version,
		ThresholdParams:  params,
		ValidFrom:        validFrom,
		Reason:           strings.TrimSpace(reason),
		CreatedBy:        strings.TrimSpace(createdBy),
		CreatedAt:        createdAt,
	}
	if v.ID, err = tx.Insert(ctx, v); err != nil {
		return entities.ThresholdVersion{}, err
	}
	if open != nil {
		previous := open.ID
		v.PreviousVersionID = &previous
	}
	return v, nil
}

// withPreviousIDs completa PreviousVersionID en un historial ascendente por versión.
func withPreviousIDs(history []entities.ThresholdVersion) []entities.ThresholdVersion {
	for n := 1; n < len(history); n++ {
		previous := history[n-1].ID
		history[n].PreviousVersionID = &previous
	}
	return history
}

func normalizeEventType(t entities.EventType) entities.EventType {
	if t == "" {
		return entities.EventTypeLeak
	}
	return entities.EventType(strings.ToUpper(string(t)))
}
