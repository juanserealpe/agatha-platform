// Package services son los casos de uso de Alert Management: procesamiento
// horario (en vivo, catch-up y backfill), gestión, consultas, exportación,
// política y eventos conocidos. No entrega avisos: publica los mensajes
// NOTIFICACION (contract.TopicAlertNotification) para el módulo de notificaciones.
package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/services"
	sharedErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// maxSynthesizedHours acota las horas sin cálculo que se infieren de un salto
// de evaluated_at (un mes): más allá, la pausa ya está registrada.
const maxSynthesizedHours = 744

// ProcessOutcome es lo que pasó con una hora.
type ProcessOutcome string

const (
	// ProcessDuplicate: la hora es igual o anterior a la última procesada (no hace nada).
	ProcessDuplicate ProcessOutcome = "DUPLICATE"
	// ProcessIgnored: tipo de evento desconocido.
	ProcessIgnored ProcessOutcome = "IGNORED"
	// ProcessDone: la hora (y los huecos inferidos antes de ella) se procesó.
	ProcessDone ProcessOutcome = "PROCESSED"
)

// ProcessResult es el resumen de una hora.
type ProcessResult struct {
	Outcome ProcessOutcome
	// Hours = 1 + horas sin cálculo inferidas de un salto.
	Hours    int
	Messages int
	// Notifications son los mensajes NOTIFICACION nuevos que se publicaron
	// (o se intentaron publicar) para el módulo de notificaciones.
	Notifications int
	// Decisions son los resultados de Decide (para el resumen del backfill).
	Decisions []domainServices.Outcome
}

// ProcessingService convierte cada hora del Rule Engine en el ciclo de la
// alerta (UC ProcessLevel): bajo un lock por punto lee la política vigente, el
// estado y la alerta activa, llama a Decide y guarda todo en UNA transacción
// (alerta, colores, mensajes, estado). Los mensajes NOTIFICACION se publican
// después del commit: un fallo al publicar nunca revierte la alerta.
type ProcessingService struct {
	store     ports.AlertStore
	policies  ports.PolicyStore
	devices   ports.DeviceDirectory
	publisher ports.NotificationPublisher
	clock     ports.Clock
	log       *logger.Logger

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func NewProcessingService(
	store ports.AlertStore,
	policies ports.PolicyStore,
	devices ports.DeviceDirectory,
	publisher ports.NotificationPublisher,
	clock ports.Clock,
	log *logger.Logger,
) *ProcessingService {
	return &ProcessingService{store: store, policies: policies, devices: devices, publisher: publisher, clock: clock, log: log, locks: map[int64]*sync.Mutex{}}
}

// Process procesa una hora. source decide si publica los avisos: BACKFILL
// nunca; LIVE y CATCH_UP sí, marcados Late si la hora tiene más de
// late_notification_max_hours de atraso.
func (s *ProcessingService) Process(ctx context.Context, notice entities.LevelNotice, source entities.Source) (ProcessResult, error) {
	if notice.MeasuringPointID <= 0 {
		return ProcessResult{}, domain.ErrInvalidMeasuringPointID
	}
	if notice.EventType == "" {
		notice.EventType = entities.EventTypeLeak
	}
	if !notice.EventType.Valid() {
		s.log.Warn("[ProcessingService] Tipo de evento desconocido; se ignora la hora",
			"measuring_point_id", notice.MeasuringPointID, "event_type", notice.EventType)
		return ProcessResult{Outcome: ProcessIgnored}, nil
	}
	notice.EvaluatedAt = notice.EvaluatedAt.UTC()

	unlock := s.lock(notice.MeasuringPointID)
	defer unlock()

	policy, err := s.policy(ctx, notice)
	if err != nil {
		return ProcessResult{}, err
	}
	device, err := s.devices.Get(ctx, notice.MeasuringPointID)
	if err != nil {
		s.log.Warn("[ProcessingService] No se pudo leer el dispositivo; los mensajes usan MP-<id>",
			"measuring_point_id", notice.MeasuringPointID, "error", err)
		device = nil
	}
	dev := entities.DeviceOrDefault(notice.MeasuringPointID, device)
	now := s.clock.Now().UTC()

	var result ProcessResult
	var pending []contract.AlertNotification
	err = s.store.Transaction(ctx, func(tx ports.AlertTx) error {
		result, pending = ProcessResult{}, nil
		state, err := tx.GetState(ctx, notice.MeasuringPointID, notice.EventType)
		if err != nil {
			return err
		}
		if state != nil && state.LastEvaluatedAt != nil && !notice.EvaluatedAt.After(*state.LastEvaluatedAt) {
			result.Outcome = ProcessDuplicate
			return nil
		}
		current := entities.ProcessingState{MeasuringPointID: notice.MeasuringPointID, EventType: notice.EventType}
		if state != nil {
			current = *state
		}

		active, err := tx.GetActive(ctx, notice.MeasuringPointID, notice.EventType)
		if err != nil {
			return err
		}

		for _, n := range withGaps(current, notice) {
			if current, active, err = s.apply(ctx, tx, current, active, n, policy, dev, source, now, &result, &pending); err != nil {
				return err
			}
		}
		result.Outcome = ProcessDone
		return tx.SaveState(ctx, current)
	})
	if err != nil {
		s.log.Error("[ProcessingService] Error guardando la hora",
			"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt, "source", source, "error", err)
		return ProcessResult{}, domain.ErrPersistingAlerts
	}
	s.publish(ctx, pending)
	return result, nil
}

// publish entrega los avisos de la hora ya confirmada. Un error se registra y
// sigue: la alerta y el mensaje quedaron guardados (alert_messages es la
// fuente de verdad para reconciliar).
func (s *ProcessingService) publish(ctx context.Context, pending []contract.AlertNotification) {
	if s.publisher == nil {
		return
	}
	for _, n := range pending {
		if err := s.publisher.PublishAlertNotification(ctx, n); err != nil {
			s.log.Error("[ProcessingService] No se pudo publicar el aviso",
				"alert", n.AlertCode, "message_id", n.MessageID, "code", n.Code, "error", err)
		}
	}
}

// withGaps antepone a la hora las horas sin cálculo que faltan desde la última
// procesada (el catch-up y el backfill no tienen fila de las NO_CALCULABLE).
func withGaps(state entities.ProcessingState, notice entities.LevelNotice) []entities.LevelNotice {
	if state.LastEvaluatedAt == nil {
		return []entities.LevelNotice{notice}
	}
	var out []entities.LevelNotice
	for t := state.LastEvaluatedAt.Add(time.Hour); t.Before(notice.EvaluatedAt) && len(out) < maxSynthesizedHours; t = t.Add(time.Hour) {
		out = append(out, entities.NoCalculable(notice.MeasuringPointID, notice.EventType, t))
	}
	return append(out, notice)
}

// apply decide y persiste una hora dentro de la transacción.
func (s *ProcessingService) apply(
	ctx context.Context,
	tx ports.AlertTx,
	state entities.ProcessingState,
	active *entities.Alert,
	notice entities.LevelNotice,
	policy entities.Policy,
	device entities.Device,
	source entities.Source,
	now time.Time,
	result *ProcessResult,
	pending *[]contract.AlertNotification,
) (entities.ProcessingState, *entities.Alert, error) {
	in := domainServices.DecideInput{State: state, Active: active, Notice: notice, Policy: policy, Device: device}

	var segment *entities.Segment
	var err error
	if active != nil {
		if segment, err = tx.GetCurrentSegment(ctx, active.ID); err != nil {
			return state, active, err
		}
		in.Segment = segment
	} else if state.LastAlertID != nil && notice.EpisodeEvent == entities.EpisodeOpened {
		last, err := tx.GetAlert(ctx, *state.LastAlertID, false)
		if err != nil && err != ports.ErrNotFound {
			return state, active, err
		}
		in.Last = last
	}

	d, err := domainServices.Decide(in)
	if err != nil {
		return state, active, err
	}
	result.Hours++
	result.Decisions = append(result.Decisions, d.Outcome)

	if d.Alert == nil {
		return d.State, active, nil
	}

	alert := *d.Alert
	if d.IsNew {
		alert.Source = source
		alert.CreatedAt = now
		id, err := tx.InsertAlert(ctx, alert)
		if err != nil {
			return state, active, err
		}
		alert.ID, alert.Code = id, entities.AlertCode(id)
	} else if err := tx.UpdateProcessing(ctx, alert); err != nil {
		return state, active, err
	}

	if d.CloseSegmentAt != nil {
		current := segment
		if current == nil {
			if current, err = tx.GetCurrentSegment(ctx, alert.ID); err != nil {
				return state, active, err
			}
		}
		if current != nil && current.EndedAt == nil {
			if err := tx.CloseSegment(ctx, current.ID, *d.CloseSegmentAt); err != nil {
				return state, active, err
			}
		}
	}

	segmentID := int64(0)
	if d.NewSegment != nil {
		seg := *d.NewSegment
		seg.AlertID = alert.ID
		if segmentID, err = tx.InsertSegment(ctx, seg); err != nil {
			return state, active, err
		}
	} else {
		current := segment
		if current == nil {
			if current, err = tx.GetCurrentSegment(ctx, alert.ID); err != nil {
				return state, active, err
			}
		}
		if current != nil {
			segmentID = current.ID
		}
	}

	late := now.Sub(notice.EvaluatedAt) > time.Duration(policy.LateNotificationMaxHours)*time.Hour
	for _, m := range d.Messages {
		m.AlertID = alert.ID
		m.SegmentID = segmentID
		m.CreatedAt = now
		id, inserted, err := tx.InsertMessage(ctx, m)
		if err != nil {
			return state, active, err
		}
		if !inserted {
			continue
		}
		result.Messages++
		if m.Kind != entities.KindNotification || source == entities.SourceBackfill {
			continue
		}
		m.ID = id
		result.Notifications++
		*pending = append(*pending, toAlertNotification(alert, m, source, late, now))
	}

	if d.Outcome == domainServices.OutcomeOpened || d.Outcome == domainServices.OutcomeReactivated || d.Outcome == domainServices.OutcomeFinished {
		s.log.Info("[ProcessingService] Alerta "+string(d.Outcome),
			"alert", alert.Code, "measuring_point_id", alert.MeasuringPointID, "evaluated_at", notice.EvaluatedAt,
			"level", alert.CurrentLevel, "max_level", alert.MaxLevel, "source", source)
	}

	if alert.State == entities.AlertActive {
		return d.State, &alert, nil
	}
	return d.State, nil, nil
}

// toAlertNotification arma el evento del contrato con la alerta ya actualizada.
func toAlertNotification(a entities.Alert, m entities.Message, source entities.Source, late bool, now time.Time) contract.AlertNotification {
	return contract.AlertNotification{
		MessageID:        m.ID,
		AlertID:          a.ID,
		AlertCode:        a.Code,
		MeasuringPointID: a.MeasuringPointID,
		EventType:        string(a.EventType),
		EvaluatedAt:      m.EvaluatedAt.UTC(),
		Code:             m.Code,
		Level:            string(m.Level),
		MaxLevel:         string(a.MaxLevel),
		AlertState:       string(a.State),
		Title:            m.Title,
		Body:             m.Body,
		IndicatorValue:   m.IndicatorValue,
		Source:           string(source),
		Late:             late,
		PolicyVersionID:  m.PolicyVersionID,
		OccurredAt:       now,
		SchemaVersion:    contract.SchemaVersion,
	}
}

// policy devuelve la versión vigente en la hora; sin ninguna usa la plantilla (ID 0) y avisa.
func (s *ProcessingService) policy(ctx context.Context, notice entities.LevelNotice) (entities.Policy, error) {
	p, err := s.policies.GetEffective(ctx, notice.MeasuringPointID, notice.EventType, notice.EvaluatedAt)
	if err != nil {
		s.log.Error("[ProcessingService] Error consultando la política", "measuring_point_id", notice.MeasuringPointID, "error", err)
		return entities.Policy{}, domain.ErrConsultingAlerts
	}
	if p == nil {
		s.log.Warn("[ProcessingService] Sin política vigente; se usa la plantilla",
			"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt)
		return entities.Policy{EventType: notice.EventType, PolicyParams: entities.DefaultPolicyParams()}, nil
	}
	return *p, nil
}

func (s *ProcessingService) lock(pointID int64) (unlock func()) {
	s.mu.Lock()
	l, ok := s.locks[pointID]
	if !ok {
		l = &sync.Mutex{}
		s.locks[pointID] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// isAppError indica si err es un error de negocio (se devuelve tal cual).
func isAppError(err error) bool {
	var appErr *sharedErrors.AppError
	return errors.As(err, &appErr)
}
