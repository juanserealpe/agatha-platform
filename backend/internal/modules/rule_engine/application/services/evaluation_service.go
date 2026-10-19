package services

import (
	"context"
	"sync"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// EvaluationService implementa EvaluateIndicator: interpreta cada hora
// calculada de 24 h con la versión de umbrales vigente en su evaluated_at,
// guarda la fila en rule_evaluations y, post-commit, publica EXCEEDED o
// CLEARED. Solo orquesta e IO: la lógica vive en domain/services.Assess.
type EvaluationService struct {
	thresholds   repositories.ThresholdRepository
	evaluations  repositories.RuleEvaluationRepository
	thresholdUCs *ThresholdService
	// publisher nil ⇒ no publica.
	publisher ports.LevelEventPublisher
	clock     ports.Clock
	log       *logger.Logger

	// locks serializa por punto: la hora siguiente necesita la fila de la anterior.
	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func NewEvaluationService(
	thresholds repositories.ThresholdRepository,
	evaluations repositories.RuleEvaluationRepository,
	thresholdUCs *ThresholdService,
	publisher ports.LevelEventPublisher,
	clock ports.Clock,
	log *logger.Logger,
) *EvaluationService {
	return &EvaluationService{
		thresholds:   thresholds,
		evaluations:  evaluations,
		thresholdUCs: thresholdUCs,
		publisher:    publisher,
		clock:        clock,
		log:          log,
		locks:        map[int64]*sync.Mutex{},
	}
}

// EvaluateIndicator interpreta la hora del Notice y publica el evento que
// corresponda. Es idempotente: una hora ya guardada no se recalcula ni se
// republica. Todos los errores devueltos son *AppError de rule_engine/domain.
func (s *EvaluationService) EvaluateIndicator(ctx context.Context, notice IndicatorEvaluatedNotice) (EvaluationResult, error) {
	return s.evaluate(ctx, notice, true)
}

// EvaluateIndicatorWithoutPublishing es EXACTAMENTE la misma interpretación,
// pero nunca publica. La usa el backfill.
func (s *EvaluationService) EvaluateIndicatorWithoutPublishing(ctx context.Context, notice IndicatorEvaluatedNotice) (EvaluationResult, error) {
	return s.evaluate(ctx, notice, false)
}

func (s *EvaluationService) evaluate(ctx context.Context, notice IndicatorEvaluatedNotice, publish bool) (EvaluationResult, error) {
	// Solo se interpreta la ventana de 24 h calculada. Las NO_CALCULABLE no
	// generan fila: las rachas cuentan horas calculadas (un hueco pausa, no reinicia).
	if notice.WindowHours != WindowHours24 || notice.Status != StatusCalculated || notice.I == nil {
		s.log.Debug("[EvaluationService] Ventana ignorada",
			"measuring_point_id", notice.MeasuringPointID, "window_hours", notice.WindowHours,
			"evaluated_at", notice.EvaluatedAt, "status", notice.Status)
		return EvaluationResult{Outcome: OutcomeIgnored}, nil
	}
	if notice.MeasuringPointID <= 0 {
		return EvaluationResult{}, domain.ErrInvalidMeasuringPointID
	}

	pointID, eventType, at := notice.MeasuringPointID, entities.EventTypeLeak, notice.EvaluatedAt.UTC()

	unlock := s.lock(pointID)
	defer unlock()

	// 1) Idempotencia.
	exists, hasLater, err := s.evaluations.GetPosition(ctx, pointID, eventType, at)
	if err != nil {
		s.log.Error("[EvaluationService] Error consultando las evaluaciones de reglas",
			"measuring_point_id", pointID, "evaluated_at", at, "error", err)
		return EvaluationResult{}, domain.ErrConsultingRuleEvaluations
	}
	if exists {
		s.log.Debug("[EvaluationService] Hora ya interpretada; no se recalcula ni se republica",
			"measuring_point_id", pointID, "evaluated_at", at)
		return EvaluationResult{Outcome: OutcomeDuplicate}, nil
	}

	// 2) Versión vigente en evaluated_at e historial de K horas calculadas.
	version, err := s.effectiveVersion(ctx, pointID, eventType, notice)
	if err != nil || version == nil {
		return EvaluationResult{Outcome: OutcomeNoThresholds}, err
	}

	history, err := s.evaluations.GetPrevious(ctx, pointID, eventType, at, version.HistoryDepth())
	if err != nil {
		s.log.Error("[EvaluationService] Error consultando el historial de reglas",
			"measuring_point_id", pointID, "evaluated_at", at, "error", err)
		return EvaluationResult{}, domain.ErrConsultingRuleEvaluations
	}

	// 3) Lógica pura + inserción.
	a := domainServices.Assess(*notice.I, version.ThresholdParams, history)
	row := entities.RuleEvaluation{
		MeasuringPointID:   pointID,
		EventType:          eventType,
		EvaluatedAt:        at,
		IndicatorValue:     *notice.I,
		RawLevel:           a.RawLevel,
		Level:              a.Level,
		Rule:               a.Rule,
		InEpisode:          a.InEpisode,
		HoursAbove:         a.HoursAbove,
		HoursInLevel:       a.HoursInLevel,
		ThresholdVersionID: version.ID,
		LowConfidence:      notice.LowConfidence,
		Emitted:            a.Emitted,
		CreatedAt:          s.clock.Now().UTC(),
	}

	id, inserted, err := s.evaluations.Save(ctx, row)
	if err != nil {
		s.log.Error("[EvaluationService] Error guardando la evaluación de reglas",
			"measuring_point_id", pointID, "evaluated_at", at, "error", err)
		return EvaluationResult{}, domain.ErrPersistingRuleEvaluations
	}
	if !inserted {
		// Otro proceso la guardó entre la consulta y el INSERT: él publica.
		return EvaluationResult{Outcome: OutcomeDuplicate}, nil
	}
	row.ID = id

	result := EvaluationResult{Outcome: OutcomeSaved, Evaluation: row, EpisodeOpened: a.EpisodeOpened, Late: hasLater}
	s.logEvaluation(row, version, publish)

	// 4–5) Post-commit: publicar según emitted, salvo llegada tardía.
	switch {
	case hasLater:
		s.log.Warn("[EvaluationService] Hora interpretada fuera de orden (hay horas posteriores): se guarda sin publicar",
			"measuring_point_id", pointID, "evaluated_at", at, "emitted", row.Emitted)
	case publish:
		result.Published = s.publishLevelEvent(ctx, row, a.PreviousLevel, version)
	}

	return result, nil
}

// effectiveVersion devuelve la versión vigente en evaluated_at. Si el punto no
// tiene ninguna, crea la versión 1 (primer uso, valid_from = now): la hora
// actual, anterior a now, queda sin interpretar (nil, nil).
func (s *EvaluationService) effectiveVersion(ctx context.Context, pointID int64, eventType entities.EventType, notice IndicatorEvaluatedNotice,
) (*entities.ThresholdVersion, error) {
	at := notice.EvaluatedAt.UTC()

	version, err := s.thresholds.GetEffectiveAt(ctx, pointID, eventType, at)
	if err != nil {
		s.log.Error("[EvaluationService] Error consultando los umbrales vigentes",
			"measuring_point_id", pointID, "evaluated_at", at, "error", err)
		return nil, domain.ErrConsultingThresholds
	}
	if version != nil {
		return version, nil
	}

	created, err := s.thresholdUCs.EnsureInitialVersion(ctx, pointID, eventType)
	if err != nil {
		return nil, err
	}
	if created {
		if version, err = s.thresholds.GetEffectiveAt(ctx, pointID, eventType, at); err != nil {
			return nil, domain.ErrConsultingThresholds
		}
	}
	if version == nil {
		s.log.Warn("[EvaluationService] Hora anterior a la primera versión de umbrales del punto: no se interpreta",
			"measuring_point_id", pointID, "evaluated_at", at)
	}
	return version, nil
}

func (s *EvaluationService) logEvaluation(row entities.RuleEvaluation, version *entities.ThresholdVersion, publish bool) {
	fields := []interface{}{
		"measuring_point_id", row.MeasuringPointID,
		"evaluated_at", row.EvaluatedAt,
		"i", row.IndicatorValue,
		"raw_level", row.RawLevel,
		"level", row.Level,
		"rule", row.Rule,
		"threshold_version", version.Version,
		"emitted", row.Emitted,
	}
	// El backfill interpreta miles de horas: su progreso lo resume el propio backfill.
	if !publish {
		s.log.Debug("[EvaluationService] Hora interpretada", fields...)
		return
	}
	s.log.Info("[EvaluationService] Hora interpretada", fields...)
}

// publishLevelEvent publica, post-commit, el evento que marca emitted y dice
// si se publicó. Un fallo al publicar nunca falla la evaluación (la fila es la
// fuente de verdad).
func (s *EvaluationService) publishLevelEvent(ctx context.Context, row entities.RuleEvaluation, previous entities.Level, version *entities.ThresholdVersion) bool {
	if s.publisher == nil || row.Emitted == entities.EmittedNone {
		return false
	}

	occurredAt := s.clock.Now().UTC()
	var err error

	switch row.Emitted {
	case entities.EmittedExceeded:
		err = s.publisher.PublishLeakThresholdExceeded(ctx, contract.LeakThresholdExceeded{
			RuleEvaluationID: row.ID,
			MeasuringPointID: row.MeasuringPointID,
			EventType:        string(row.EventType),
			EvaluatedAt:      row.EvaluatedAt,
			Level:            string(row.Level),
			PreviousLevel:    string(previous),
			Rule:             string(row.Rule),
			IndicatorValue:   row.IndicatorValue,
			Thresholds: contract.Thresholds{
				Amarillo: version.Amarillo,
				Naranja:  version.Naranja,
				Rojo:     version.Rojo,
			},
			ThresholdVersionID: row.ThresholdVersionID,
			HoursAbove:         row.HoursAbove,
			HoursInLevel:       row.HoursInLevel,
			LowConfidence:      row.LowConfidence,
			OccurredAt:         occurredAt,
			SchemaVersion:      contract.SchemaVersion,
		})
	case entities.EmittedCleared:
		err = s.publisher.PublishLeakThresholdCleared(ctx, contract.LeakThresholdCleared{
			RuleEvaluationID:   row.ID,
			MeasuringPointID:   row.MeasuringPointID,
			EventType:          string(row.EventType),
			EvaluatedAt:        row.EvaluatedAt,
			IndicatorValue:     row.IndicatorValue,
			LastLevel:          string(previous),
			HoursAbove:         row.HoursAbove,
			ThresholdVersionID: row.ThresholdVersionID,
			OccurredAt:         occurredAt,
			SchemaVersion:      contract.SchemaVersion,
		})
	}

	if err != nil {
		s.log.Warn("[EvaluationService] No se pudo publicar el evento de nivel",
			"measuring_point_id", row.MeasuringPointID, "evaluated_at", row.EvaluatedAt, "emitted", row.Emitted, "error", err)
	}
	return err == nil
}

func (s *EvaluationService) lock(pointID int64) (unlock func()) {
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
