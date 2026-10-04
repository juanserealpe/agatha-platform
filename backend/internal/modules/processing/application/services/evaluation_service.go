package services

import (
	"context"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// EvaluationService implementa UC-P1 (EvaluatePending) y UC-P2 (Reconcile).
// Solo orquesta e IO: la lógica vive en SlotPlanner, SliceWindow y WindowEvaluator.
type EvaluationService struct {
	reader      repositories.MeasurementReader
	evaluations repositories.EvaluationRepository
	baseline    ports.BaselineProvider
	clock       ports.Clock
	planner     domainServices.SlotPlanner
	evaluator   domainServices.WindowEvaluator
	cfg         Config
	log         *logger.Logger

	// locks serializa, por punto, el worker del bus y el reconciliador.
	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func NewEvaluationService(
	reader repositories.MeasurementReader,
	evaluations repositories.EvaluationRepository,
	baseline ports.BaselineProvider,
	clock ports.Clock,
	cfg Config,
	log *logger.Logger,
) *EvaluationService {
	return &EvaluationService{
		reader:      reader,
		evaluations: evaluations,
		baseline:    baseline,
		clock:       clock,
		planner:     domainServices.NewSlotPlanner(cfg.InitialLookbackHours),
		evaluator: domainServices.NewWindowEvaluator(domainServices.EvaluatorConfig{
			ConfigVersion: cfg.ConfigVersion,
			SilentPeriod:  cfg.SilentPeriod,
		}),
		cfg:   cfg,
		log:   log,
		locks: map[int64]*sync.Mutex{},
	}
}

// EvaluatePending (UC-P1) evalúa, en orden, las horas pendientes del punto
// hasta ReadyUntil y las guarda en una sola TX. Ante un fallo de lectura o de
// línea base aborta SIN persistir: las filas son inmutables y un fallo
// transitorio no debe quedar grabado como NO_CALCULABLE (el reconciliador
// reintenta). Todos los errores devueltos son *AppError de processing/domain.
func (s *EvaluationService) EvaluatePending(ctx context.Context, cmd EvaluatePendingCommand,
) (EvaluationResult, error) {
	result := EvaluationResult{PointID: cmd.PointID}

	if cmd.PointID <= 0 {
		s.log.Error("[EvaluationService] Punto de medición inválido", "measuring_point_id", cmd.PointID)
		return result, domain.ErrInvalidMeasuringPointID
	}

	unlock := s.lock(cmd.PointID)
	defer unlock()

	now := s.clock.Now().UTC()

	readyUntil, ok, err := s.resolveReadyUntil(ctx, cmd, now)
	if err != nil || !ok {
		return result, err
	}
	result.ReadyUntil = readyUntil

	last, err := s.evaluations.GetLastEvaluatedAt(ctx, cmd.PointID)
	if err != nil {
		s.log.Error("[EvaluationService] Error consultando la última hora evaluada",
			"measuring_point_id", cmd.PointID, "error", err)
		return result, domain.ErrConsultingEvaluations
	}

	s.warnLateData(cmd, last)

	slots := s.planner.Plan(last, readyUntil, cmd.MaxSlots)
	if len(slots) == 0 {
		return result, nil
	}

	first, end := slots[0], slots[len(slots)-1]
	result.Planned = len(slots)
	result.FirstSlot, result.LastSlot = &first, &end

	evals, err := s.evaluate(ctx, cmd, slots, now)
	if err != nil {
		return result, err
	}

	inserted, err := s.evaluations.SaveAll(ctx, evals)
	if err != nil {
		s.log.Error("[EvaluationService] Error guardando las evaluaciones",
			"measuring_point_id", cmd.PointID, "first_slot", first, "last_slot", end, "error", err)
		return result, domain.ErrPersistingEvaluations
	}
	result.Inserted = inserted

	if inserted < len(evals) {
		s.log.Info("[EvaluationService] Otro proceso ya había insertado parte de las evaluaciones",
			"measuring_point_id", cmd.PointID, "rows", len(evals), "inserted", inserted)
	}

	s.warnLostEvent(cmd, last, evals)

	s.log.Info("[EvaluationService] Horas evaluadas",
		"measuring_point_id", cmd.PointID,
		"trigger", cmd.Trigger,
		"hours", len(slots),
		"inserted", inserted,
		"first_slot", first,
		"last_slot", end,
	)

	return result, nil
}

// Reconcile (UC-P2) evalúa las horas pendientes de cada punto activo. Es la
// red de seguridad: cubre eventos perdidos, dispositivos sin datos y caídas.
// Un error en un punto se registra y no detiene a los demás; solo falla si no
// se pueden listar los puntos.
func (s *EvaluationService) Reconcile(ctx context.Context, now time.Time) ([]PointReconciliation, error) {
	points, err := s.reader.ListActiveMeasuringPoints(ctx)
	if err != nil {
		s.log.Error("[EvaluationService] Error listando puntos activos", "error", err)
		return nil, domain.ErrConsultingMeasurements
	}

	summary := make([]PointReconciliation, 0, len(points))

	for _, pointID := range points {
		if ctx.Err() != nil {
			s.log.Warn("[EvaluationService] Reconciliación interrumpida", "error", ctx.Err())
			break
		}

		item := PointReconciliation{PointID: pointID}

		latest, err := s.reader.GetLatestTimestamp(ctx, pointID)
		if err != nil {
			s.log.Error("[EvaluationService] Error consultando el último dato del punto",
				"measuring_point_id", pointID, "error", err)
			item.Err = domain.ErrConsultingMeasurements
			summary = append(summary, item)
			continue
		}

		item.Result, item.Err = s.EvaluatePending(ctx, EvaluatePendingCommand{
			PointID:    pointID,
			ReadyUntil: reconcileReadyUntil(now, latest, s.cfg.Grace),
			Trigger:    entities.TriggerReconciler,
			MaxSlots:   s.cfg.MaxSlotsReconciler,
		})
		summary = append(summary, item)
	}

	return summary, nil
}

// reconcileReadyUntil = min(floorHour(now), max(floorHour(maxDataTs), floorHour(now − grace))):
// una hora está lista si ya llegó un dato en o después de ella o si pasó el grace.
func reconcileReadyUntil(now time.Time, latest *time.Time, grace time.Duration) time.Time {
	ceiling := floorHour(now)

	ready := floorHour(now.Add(-grace))
	if latest != nil && floorHour(*latest).After(ready) {
		ready = floorHour(*latest)
	}

	if ready.After(ceiling) {
		return ceiling
	}
	return ready
}

// resolveReadyUntil aplica el tope floorHour(now) y, si el comando no trae
// ReadyUntil, lo deriva del último dato. ok = false ⇒ no hay nada que evaluar.
func (s *EvaluationService) resolveReadyUntil(ctx context.Context, cmd EvaluatePendingCommand, now time.Time,
) (time.Time, bool, error) {
	ready := cmd.ReadyUntil

	if ready.IsZero() {
		latest, err := s.reader.GetLatestTimestamp(ctx, cmd.PointID)
		if err != nil {
			s.log.Error("[EvaluationService] Error consultando el último dato del punto",
				"measuring_point_id", cmd.PointID, "error", err)
			return time.Time{}, false, domain.ErrConsultingMeasurements
		}
		if latest == nil {
			return time.Time{}, false, nil
		}
		ready = *latest
	}

	ready = floorHour(ready)
	if ceiling := floorHour(now); ready.After(ceiling) {
		ready = ceiling
	}

	return ready, true, nil
}

// evaluate lee una vez los samples de [primera − 24 h, última) y evalúa cada
// hora en orden. Cualquier fallo aborta el lote completo.
func (s *EvaluationService) evaluate(ctx context.Context, cmd EvaluatePendingCommand, slots []time.Time, now time.Time,
) ([]entities.FeatureEvaluation, error) {
	from := slots[0].Add(-entities.Window24h * time.Hour)
	to := slots[len(slots)-1]

	raw, err := s.reader.GetSamples(ctx, cmd.PointID, from, to)
	if err != nil {
		s.log.Error("[EvaluationService] Error consultando las mediciones",
			"measuring_point_id", cmd.PointID, "from", from, "to", to, "error", err)
		return nil, domain.ErrConsultingMeasurements
	}
	samples := features.Normalize(raw)

	evals := make([]entities.FeatureEvaluation, 0, 2*len(slots))

	for _, t := range slots {
		snapshot, err := s.baseline.GetEffective(ctx, cmd.PointID, t)
		if err != nil {
			s.log.Error("[EvaluationService] Error consultando la línea base vigente",
				"measuring_point_id", cmd.PointID, "at", t, "error", err)
			return nil, domain.ErrConsultingBaseline
		}

		w24, w6 := s.evaluator.Evaluate(domainServices.EvaluationInput{
			PointID:   cmd.PointID,
			At:        t,
			Window24:  domainServices.SliceWindow(samples, t, entities.Window24h),
			Window6:   domainServices.SliceWindow(samples, t, entities.Window6h),
			Baseline:  snapshot,
			Trigger:   cmd.Trigger,
			BatchSeq:  cmd.BatchSeq,
			CreatedAt: now,
		})
		evals = append(evals, w24, w6)
	}

	return evals, nil
}

// warnLateData: un registro anterior a la última hora evaluada ya no cambia
// ninguna evaluación (son inmutables).
func (s *EvaluationService) warnLateData(cmd EvaluatePendingCommand, last *time.Time) {
	if cmd.DataFrom == nil || last == nil || !cmd.DataFrom.Before(*last) {
		return
	}
	s.log.Warn("[EvaluationService] Llegaron datos de horas ya evaluadas; las evaluaciones no se modifican",
		"measuring_point_id", cmd.PointID, "data_from", cmd.DataFrom.UTC(), "last_evaluated_at", *last)
}

// warnLostEvent: si el reconciliador evalúa horas que ya tenían datos, el
// evento correspondiente no llegó (mide la fiabilidad del bus). Sin historial
// previo no aplica: es la evaluación inicial.
func (s *EvaluationService) warnLostEvent(cmd EvaluatePendingCommand, last *time.Time, evals []entities.FeatureEvaluation) {
	if cmd.Trigger != entities.TriggerReconciler || last == nil {
		return
	}

	withData := 0
	for _, e := range evals {
		if e.WindowHours == entities.Window24h && e.NValid > 0 {
			withData++
		}
	}

	if withData > 0 {
		s.log.Warn("[EvaluationService] El reconciliador evaluó horas con datos: posible evento perdido",
			"measuring_point_id", cmd.PointID, "hours_with_data", withData)
	}
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

func floorHour(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}
