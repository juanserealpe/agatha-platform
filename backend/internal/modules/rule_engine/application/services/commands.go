package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// IndicatorEvaluatedNotice es la vista propia del Rule Engine de una ventana
// (24 h o 6 h) evaluada por Processing. internal/bootstrap la traduce del
// contrato de Processing (el módulo no lo importa). Es una pista: la fila de
// feature_evaluations (punto, ventana, hora) es la fuente de verdad.
type IndicatorEvaluatedNotice struct {
	MeasuringPointID int64
	WindowHours      int       // 24 | 6
	EvaluatedAt      time.Time // fin exclusivo de la ventana, hora en punto UTC
	Status           string    // CALCULATED | NO_CALCULABLE
	NoCalcReason     string    // vacío si Status = CALCULATED
	I                *float64  // nil si no se calculó
	LowConfidence    bool
	Trigger          string
	OccurredAt       time.Time
}

// Valores del Notice que interpreta el Rule Engine.
const (
	WindowHours24      = 24
	StatusCalculated   = "CALCULATED"
	StatusNoCalculable = "NO_CALCULABLE"
	TriggerBackfill    = "BACKFILL"
	TriggerSimulation  = "SIMULATION"
	// TriggerCatchUp: hora interpretada por el catch-up (no llegó su evento).
	TriggerCatchUp = "CATCH_UP"
)

// CreateThresholdVersionCommand es la entrada de CreateThresholdVersion.
type CreateThresholdVersionCommand struct {
	PointID   int64
	EventType entities.EventType // vacío ⇒ FUGA
	// Patch: los campos no informados se copian de la versión vigente.
	Patch entities.ThresholdPatch
	// ValidFrom cero ⇒ now. Nunca en el pasado.
	ValidFrom time.Time
	Reason    string
	CreatedBy string
}

// CreateThresholdVersionBulkCommand es la entrada de CreateThresholdVersionBulk.
type CreateThresholdVersionBulkCommand struct {
	// PointIDs vacío ⇒ todos los puntos que ya tienen alguna versión.
	PointIDs  []int64
	EventType entities.EventType
	Patch     entities.ThresholdPatch
	ValidFrom time.Time
	Reason    string
	CreatedBy string
}

// EvaluationOutcome es lo que pasó con un Notice.
type EvaluationOutcome string

const (
	// OutcomeIgnored: ventana de 6 h, estado desconocido o CALCULATED sin I (no genera fila).
	OutcomeIgnored EvaluationOutcome = "IGNORED"
	// OutcomeNoCalculable: hora NO_CALCULABLE de 24 h; no genera fila y se
	// reenvía como level_evaluated (salvo tardía o en backfill).
	OutcomeNoCalculable EvaluationOutcome = "NO_CALCULABLE"
	// OutcomeDuplicate: la hora ya tenía fila; no se recalcula ni se republica.
	OutcomeDuplicate EvaluationOutcome = "DUPLICATE"
	// OutcomeNoThresholds: la hora es anterior a la primera versión de umbrales del punto.
	OutcomeNoThresholds EvaluationOutcome = "NO_THRESHOLDS"
	// OutcomeSaved: fila nueva guardada.
	OutcomeSaved EvaluationOutcome = "SAVED"
)

// EvaluationResult es la salida de EvaluateIndicator.
type EvaluationResult struct {
	Outcome EvaluationOutcome
	// Evaluation es la fila guardada (solo con OutcomeSaved).
	Evaluation entities.RuleEvaluation
	// Late: había filas posteriores del punto (no se publicó).
	Late bool
	// Published: se publicó la level_evaluated de la hora.
	Published bool
}
