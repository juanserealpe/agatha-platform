package contract

import "time"

// Thresholds son los umbrales de la versión con la que se interpretó la hora.
type Thresholds struct {
	Amarillo float64 `json:"amarillo"`
	Naranja  float64 `json:"naranja"`
	Rojo     float64 `json:"rojo"`
}

// LeakThresholdExceeded indica que, en la hora EvaluatedAt, el punto está en
// un episodio de fuga por encima del umbral. Llega uno por cada hora calculada
// del episodio (incluida la que lo abre); no llega en la racha de entrada
// pendiente ni en la banda de histéresis. Todos los time.Time están en UTC.
type LeakThresholdExceeded struct {
	// RuleEvaluationID es el id de la fila de rule_evaluations.
	RuleEvaluationID int64 `json:"rule_evaluation_id"`
	MeasuringPointID int64 `json:"measuring_point_id"`
	// EventType es el tipo de evento interpretado ("FUGA").
	EventType string `json:"event_type"`
	// EvaluatedAt es el fin exclusivo de la ventana de 24 h (hora en punto).
	EvaluatedAt time.Time `json:"evaluated_at"`
	// Level es AMARILLO, NARANJA o ROJO.
	Level string `json:"level"`
	// PreviousLevel es el nivel de la hora calculada anterior (NORMAL|AMARILLO|NARANJA|ROJO).
	PreviousLevel string `json:"previous_level"`
	// Rule es THRESHOLD o PERSISTENCE.
	Rule               string     `json:"rule"`
	IndicatorValue     float64    `json:"indicator_value"`
	Thresholds         Thresholds `json:"thresholds"`
	ThresholdVersionID int64      `json:"threshold_version_id"`
	// HoursAbove son las horas calculadas del episodio, incluida esta.
	HoursAbove int `json:"hours_above"`
	// HoursInLevel son las horas calculadas consecutivas en Level.
	HoursInLevel  int       `json:"hours_in_level"`
	LowConfidence bool      `json:"low_confidence"`
	OccurredAt    time.Time `json:"occurred_at"`
	SchemaVersion int       `json:"schema_version"`
}

// LeakThresholdCleared indica que el episodio de fuga terminó en la hora
// EvaluatedAt (se cumplió la racha de salida). Llega una sola vez por episodio.
type LeakThresholdCleared struct {
	RuleEvaluationID int64     `json:"rule_evaluation_id"`
	MeasuringPointID int64     `json:"measuring_point_id"`
	EventType        string    `json:"event_type"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	IndicatorValue   float64   `json:"indicator_value"`
	// LastLevel es el nivel del episodio en la hora anterior al cierre.
	LastLevel string `json:"last_level"`
	// HoursAbove es la duración del episodio en horas calculadas, incluida la de cierre.
	HoursAbove         int       `json:"hours_above"`
	ThresholdVersionID int64     `json:"threshold_version_id"`
	OccurredAt         time.Time `json:"occurred_at"`
	SchemaVersion      int       `json:"schema_version"`
}
