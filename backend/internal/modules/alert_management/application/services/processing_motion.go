package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/services"
)

// ProcessMotion procesa una hora de MOVIMIENTO sobre la alerta de MOVIMIENTO
// del punto, bajo el mismo lock por punto que FUGA (§7.2). FUGA y MOVIMIENTO
// nunca comparten alerta: si se solapan son dos alertas. Lo único que el
// movimiento comparte es la temperatura: sus TEMP_* se replican en la alerta
// de FUGA vigente (DecideTemperature), con el mismo tipo de mensaje.
//
// Cada flujo tiene su propio last_evaluated_at: una hora igual o anterior a
// la última del flujo no hace nada. Una hora en modo sombra (alerts_enabled =
// false) se ignora. La política es la de FUGA (mismas reglas de reactivación
// y de atraso). En vivo, si faltan horas desde la última procesada (evento
// perdido), se leen de motion_evaluations antes de la recibida.
func (s *ProcessingService) ProcessMotion(ctx context.Context, notice entities.MotionNotice, source entities.Source) (ProcessResult, error) {
	if notice.MeasuringPointID <= 0 {
		return ProcessResult{}, domain.ErrInvalidMeasuringPointID
	}
	if !notice.AlertsEnabled {
		return ProcessResult{Outcome: ProcessIgnored}, nil
	}
	notice.EvaluatedAt = notice.EvaluatedAt.UTC()

	unlock := s.lock(notice.MeasuringPointID)
	defer unlock()

	policy, err := s.policy(ctx, entities.LevelNotice{
		MeasuringPointID: notice.MeasuringPointID, EventType: entities.EventTypeLeak, EvaluatedAt: notice.EvaluatedAt,
	})
	if err != nil {
		return ProcessResult{}, err
	}
	device, err := s.devices.Get(ctx, notice.MeasuringPointID)
	if err != nil {
		device = nil
	}
	dev := entities.DeviceOrDefault(notice.MeasuringPointID, device)
	now := s.clock.Now().UTC()

	var result ProcessResult
	var pending []contract.AlertNotification
	var changed []contract.AlertChanged
	err = s.store.Transaction(ctx, func(tx ports.AlertTx) error {
		result, pending, changed = ProcessResult{}, nil, nil
		state, err := tx.GetState(ctx, notice.MeasuringPointID, entities.EventTypeMovement)
		if err != nil {
			return err
		}
		if state != nil && state.LastEvaluatedAt != nil && !notice.EvaluatedAt.After(*state.LastEvaluatedAt) {
			result.Outcome = ProcessDuplicate
			return nil
		}
		current := entities.ProcessingState{MeasuringPointID: notice.MeasuringPointID, EventType: entities.EventTypeMovement}
		if state != nil {
			current = *state
		}
		active, err := tx.GetActive(ctx, notice.MeasuringPointID, entities.EventTypeMovement)
		if err != nil {
			return err
		}
		leak, err := tx.GetActive(ctx, notice.MeasuringPointID, entities.EventTypeLeak)
		if err != nil {
			return err
		}

		hours, err := s.missingMotionHours(ctx, current, notice, source)
		if err != nil {
			return err
		}
		for _, n := range hours {
			if current, active, err = s.applyMotion(ctx, tx, current, active, n, policy, dev, source, now, &result, &pending, &changed); err != nil {
				return err
			}
			if leak, err = s.applyTemperature(ctx, tx, leak, n, policy, dev, source, now, &result, &pending, &changed); err != nil {
				return err
			}
		}
		result.Outcome = ProcessDone
		return tx.SaveState(ctx, current)
	})
	if err != nil {
		s.log.Error("[ProcessingService] Error guardando la hora de movimiento",
			"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt, "source", source, "error", err)
		return ProcessResult{}, domain.ErrPersistingAlerts
	}
	s.publish(ctx, pending)
	s.publishChanges(ctx, changed)
	return result, nil
}

// missingMotionHours antepone a la hora las de motion_evaluations que faltan
// desde la última procesada (solo en vivo y con fuente).
func (s *ProcessingService) missingMotionHours(ctx context.Context, state entities.ProcessingState, notice entities.MotionNotice, source entities.Source) ([]entities.MotionNotice, error) {
	if s.motions == nil || source == entities.SourceBackfill || state.LastEvaluatedAt == nil ||
		!notice.EvaluatedAt.After(state.LastEvaluatedAt.Add(time.Hour)) {
		return []entities.MotionNotice{notice}, nil
	}
	missing, err := s.motions.ListAfter(ctx, notice.MeasuringPointID, *state.LastEvaluatedAt, notice.EvaluatedAt, maxSynthesizedHours)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		s.log.Warn("[ProcessingService] Horas de movimiento sin evento: se leen de motion_evaluations",
			"measuring_point_id", notice.MeasuringPointID, "hours", len(missing), "until", notice.EvaluatedAt)
	}
	return append(missing, notice), nil
}

func (s *ProcessingService) applyMotion(
	ctx context.Context,
	tx ports.AlertTx,
	state entities.ProcessingState,
	active *entities.Alert,
	notice entities.MotionNotice,
	policy entities.Policy,
	device entities.Device,
	source entities.Source,
	now time.Time,
	result *ProcessResult,
	pending *[]contract.AlertNotification,
	changed *[]contract.AlertChanged,
) (entities.ProcessingState, *entities.Alert, error) {
	in := domainServices.MotionInput{State: state, Active: active, Notice: notice, Policy: policy, Device: device}
	var segment *entities.Segment
	var err error
	if active != nil {
		if segment, err = tx.GetCurrentSegment(ctx, active.ID); err != nil {
			return state, active, err
		}
		in.Segment = segment
	} else if in.Last, err = tx.GetLastFinished(ctx, notice.MeasuringPointID, entities.EventTypeMovement); err != nil {
		return state, active, err
	}

	d, err := domainServices.DecideMotion(in)
	if err != nil {
		return state, active, err
	}
	result.Hours++
	result.Decisions = append(result.Decisions, d.Outcome)
	return s.persist(ctx, tx, d, segment, active, notice.EvaluatedAt, policy, source, now, result, pending, changed)
}

// applyTemperature replica los TEMP_* de la hora en la alerta de FUGA vigente
// (leak, nil si no hay). Devuelve la alerta de FUGA actualizada.
func (s *ProcessingService) applyTemperature(
	ctx context.Context,
	tx ports.AlertTx,
	leak *entities.Alert,
	notice entities.MotionNotice,
	policy entities.Policy,
	device entities.Device,
	source entities.Source,
	now time.Time,
	result *ProcessResult,
	pending *[]contract.AlertNotification,
	changed *[]contract.AlertChanged,
) (*entities.Alert, error) {
	if leak == nil || !domainServices.HasTemperatureReason(notice) {
		return leak, nil
	}
	d, err := domainServices.DecideTemperature(domainServices.TemperatureInput{Alert: *leak, Notice: notice, Policy: policy, Device: device})
	if err != nil || d.Alert == nil {
		return leak, err
	}
	segment, err := tx.GetCurrentSegment(ctx, leak.ID)
	if err != nil {
		return leak, err
	}
	_, updated, err := s.persist(ctx, tx, d, segment, leak, notice.EvaluatedAt, policy, source, now, result, pending, changed)
	return updated, err
}
