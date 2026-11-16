package entities

import "time"

// Cuts son tres valores de I, uno por nivel (umbrales o salidas).
type Cuts struct {
	Amarillo, Naranja, Rojo float64
}

// For devuelve el valor del nivel (0 para NORMAL).
func (c Cuts) For(l Level) float64 {
	switch l {
	case LevelAmarillo:
		return c.Amarillo
	case LevelNaranja:
		return c.Naranja
	case LevelRojo:
		return c.Rojo
	default:
		return 0
	}
}

// LevelNotice es la vista propia de Alert Management de una hora de 24 h
// interpretada por el Rule Engine (rule_engine.level_evaluated v3, o una fila
// de rule_evaluations en el catch-up/backfill). internal/bootstrap la traduce:
// el módulo no importa al Rule Engine.
type LevelNotice struct {
	RuleEvaluationID int64
	MeasuringPointID int64
	EventType        EventType
	// EvaluatedAt es el fin exclusivo de la ventana (hora en punto, UTC).
	EvaluatedAt time.Time
	// Status es CALCULATED o NO_CALCULABLE.
	Status       string
	NoCalcReason string
	// IndicatorValue es nil con NO_CALCULABLE.
	IndicatorValue *float64

	RawLevel      Level
	ComputedLevel Level
	EpisodeLevel  Level
	InEpisode     bool
	EpisodeEvent  EpisodeEvent
	// PersistenceHours > 0 si la persistencia subió el computed_level en esta
	// hora (motivo LEAK_PERSISTENCE): distingue "subió por duración" de "de golpe".
	PersistenceHours int

	Thresholds         Cuts
	Exit               Cuts
	ThresholdVersionID int64
	HoursAbove         int

	RunsAmarillo, RunsNaranja int
	// EscalationAmarillo / EscalationNaranja: horas de racha que suben un nivel (nil = desactivado).
	EscalationAmarillo, EscalationNaranja *int
	// BelowRun son las horas seguidas bajo la salida de AMARILLO, incluida esta.
	BelowRun int

	LowConfidence bool

	// DPl es el % de registros de las últimas 24 h con planitud alta (nil si
	// no se calculó). DPlMin es el mínimo de la versión (nil si no lo define:
	// la regla de planitud no está activa para el punto).
	DPl, DPlMin *float64
	// LimitedFrom es el nivel que el índice daría solo cuando el Rule Engine
	// limitó la hora a Amarillo por la planitud (motivo NARANJA_NOT_CONFIRMED,
	// Naranja o Rojo); vacío si la hora no se limitó.
	LimitedFrom Level
}

// Limited indica que la planitud limitó la hora a Amarillo.
func (n LevelNotice) Limited() bool {
	return n.LimitedFrom != ""
}

// Calculated indica si la hora tiene I.
func (n LevelNotice) Calculated() bool {
	return n.Status == StatusCalculated && n.IndicatorValue != nil
}

// Value es el I de la hora (0 si no se calculó).
func (n LevelNotice) Value() float64 {
	if n.IndicatorValue == nil {
		return 0
	}
	return *n.IndicatorValue
}

// NoCalculable crea la hora sin cálculo que el catch-up o el backfill
// infieren de un salto de evaluated_at (las NO_CALCULABLE no tienen fila).
func NoCalculable(pointID int64, eventType EventType, at time.Time) LevelNotice {
	return LevelNotice{MeasuringPointID: pointID, EventType: eventType, EvaluatedAt: at, Status: StatusNoCalculable}
}
