package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// BackfillCommand es la entrada de UC-P3.
type BackfillCommand struct {
	PointID int64
	From    time.Time // cero ⇒ primera hora con línea base
	To      time.Time // cero ⇒ última hora lista (la misma que usaría el reconciliador)
	DryRun  bool      // evalúa sin persistir
}

// BackfillResult es la salida de UC-P3. Evaluations trae las filas evaluadas
// (dos por hora), se hayan persistido o no.
type BackfillResult struct {
	PointID int64
	// NoBaseline = true ⇒ el punto no tiene línea base: no hay nada que evaluar.
	NoBaseline          bool
	FirstSlot, LastSlot *time.Time
	Hours               int
	Inserted            int // filas nuevas; el resto ya existía
	Evaluations         []entities.FeatureEvaluation
}

// Backfill (UC-P3) carga el histórico de evaluaciones: evalúa TODAS las horas
// de [From, To] con la versión de línea base que estaba vigente en cada una y
// las guarda en una sola TX.
//
// Es seguro repetirlo y correrlo con el reconciliador activo: las filas son
// inmutables y se insertan con ON CONFLICT DO NOTHING, así que solo completa
// las horas que faltan. Para que el histórico tenga sentido, el de Baseline
// (cmd/baseline -backfill) debe estar cargado antes: una hora anterior a la
// primera versión de línea base no se evalúa.
func (s *EvaluationService) Backfill(ctx context.Context, cmd BackfillCommand) (BackfillResult, error) {
	result := BackfillResult{PointID: cmd.PointID, Evaluations: []entities.FeatureEvaluation{}}

	if cmd.PointID <= 0 {
		s.log.Error("[EvaluationService] Punto de medición inválido", "measuring_point_id", cmd.PointID)
		return result, domain.ErrInvalidMeasuringPointID
	}

	unlock := s.lock(cmd.PointID)
	defer unlock()

	now := s.clock.Now().UTC()

	latest, err := s.reader.GetLatestTimestamp(ctx, cmd.PointID)
	if err != nil {
		s.log.Error("[EvaluationService] Error consultando el último dato del punto",
			"measuring_point_id", cmd.PointID, "error", err)
		return result, domain.ErrConsultingMeasurements
	}

	// Nunca más allá de la última hora lista: una hora incompleta quedaría
	// grabada para siempre con los datos que hubiera en ese momento.
	to := reconcileReadyUntil(now, latest, s.cfg.Grace)
	if !cmd.To.IsZero() && floorHour(cmd.To).Before(to) {
		to = floorHour(cmd.To)
	}

	from, ok, err := s.earliestSlot(ctx, cmd.PointID)
	if err != nil {
		return result, err
	}
	if !ok {
		result.NoBaseline = true
		return result, nil
	}
	if !cmd.From.IsZero() && cmd.From.After(from) {
		from = ceilHour(cmd.From)
	}

	slots := s.planner.Range(from, to)
	if len(slots) == 0 {
		return result, nil
	}

	first, end := slots[0], slots[len(slots)-1]
	result.FirstSlot, result.LastSlot = &first, &end
	result.Hours = len(slots)

	evals, err := s.evaluate(ctx, cmd.PointID, slots, entities.TriggerBackfill, nil, now)
	if err != nil {
		return result, err
	}
	result.Evaluations = evals

	if cmd.DryRun {
		return result, nil
	}

	inserted, err := s.evaluations.SaveAll(ctx, evals)
	if err != nil {
		s.log.Error("[EvaluationService] Error guardando el histórico de evaluaciones",
			"measuring_point_id", cmd.PointID, "first_slot", first, "last_slot", end, "error", err)
		return result, domain.ErrPersistingEvaluations
	}
	result.Inserted = inserted

	s.log.Info("[EvaluationService] Histórico de evaluaciones cargado",
		"measuring_point_id", cmd.PointID,
		"hours", len(slots),
		"rows", len(evals),
		"inserted", inserted,
		"first_slot", first,
		"last_slot", end,
	)

	return result, nil
}
