package contract

import "time"

// LevelCuts son tres valores de I, uno por nivel: los umbrales de entrada
// (Thresholds) o las salidas por histéresis (Exit = corte − exit_margin).
type LevelCuts struct {
	Amarillo float64 `json:"amarillo"`
	Naranja  float64 `json:"naranja"`
	Rojo     float64 `json:"rojo"`
}

// ReasonData son los datos de un motivo. Solo vienen los del código:
//   - LEAK_THRESHOLD: I y Threshold (corte del nivel del motivo).
//   - LEAK_PERSISTENCE: Hours (racha) y From (nivel sobre el que se contó la racha).
type ReasonData struct {
	I         float64 `json:"i,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Hours     int     `json:"hours,omitempty"`
	From      string  `json:"from,omitempty"`
}

// Reason explica el ComputedLevel de la hora (umbral y, si aplica, persistencia).
type Reason struct {
	Code  string     `json:"code"`
	Level string     `json:"level"`
	Data  ReasonData `json:"data"`
}

// LevelEvaluated es la interpretación de UNA hora de la ventana de 24 h.
// Todos los time.Time están en UTC.
//
// Con Status = NO_CALCULABLE no hay fila: RuleEvaluationID = 0, IndicatorValue
// = nil, niveles y EpisodeEvent vacíos, sin Reasons ni umbrales. Distingue
// "sin datos" de "bajo el umbral"; qué hacer con la hora lo decide el consumidor.
type LevelEvaluated struct {
	// RuleEvaluationID es el id de la fila de rule_evaluations (0 si NO_CALCULABLE).
	RuleEvaluationID int64 `json:"rule_evaluation_id"`
	MeasuringPointID int64 `json:"measuring_point_id"`
	// EventType es el tipo de evento interpretado ("FUGA").
	EventType string `json:"event_type"`
	// EvaluatedAt es el fin exclusivo de la ventana de 24 h (hora en punto).
	EvaluatedAt time.Time `json:"evaluated_at"`
	// Status es CALCULATED o NO_CALCULABLE.
	Status string `json:"status"`
	// NoCalcReason es el motivo de Processing (solo con NO_CALCULABLE).
	NoCalcReason   string   `json:"no_calc_reason,omitempty"`
	IndicatorValue *float64 `json:"indicator_value"`

	// RawLevel es el nivel que marca el umbral, sin reglas.
	RawLevel string `json:"raw_level"`
	// ComputedLevel = max(raw, persistencia), sin histéresis ni racha de
	// entrada: el nivel que sostiene la evidencia en ESTA hora.
	ComputedLevel string `json:"computed_level"`
	// EpisodeLevel es el nivel del episodio del Rule Engine (con HOLD).
	EpisodeLevel string `json:"episode_level"`
	InEpisode    bool   `json:"in_episode"`
	// EpisodeEvent es OPENED (esta hora abrió el episodio), CLEARED (lo cerró) o NONE.
	EpisodeEvent string   `json:"episode_event"`
	Reasons      []Reason `json:"reasons"`

	Thresholds LevelCuts `json:"thresholds"`
	// Exit son las salidas por nivel: corte − exit_margin.
	Exit               LevelCuts `json:"exit"`
	ThresholdVersionID int64     `json:"threshold_version_id"`
	// HoursAbove son las horas calculadas del episodio, incluida esta (0 fuera de él).
	HoursAbove    int       `json:"hours_above"`
	LowConfidence bool      `json:"low_confidence"`
	OccurredAt    time.Time `json:"occurred_at"`
	SchemaVersion int       `json:"schema_version"`
}
