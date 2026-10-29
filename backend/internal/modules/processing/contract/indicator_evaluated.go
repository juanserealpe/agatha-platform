package contract

import "time"

// IndicatorEvaluated indica que quedó persistida (post-commit) la evaluación
// del índice I de UNA ventana de un punto: hay un evento para la de 24 h y
// otro para la de 6 h de cada hora evaluada.
//
// Es una PISTA: la fila de feature_evaluations (punto, ventana, hora) es la
// fuente de verdad. Lleva el resultado (Status, I) porque es el hecho que
// interesa a los consumidores, no los componentes ni los scores.
// Todos los time.Time están en UTC.
type IndicatorEvaluated struct {
	// MeasuringPointID es el punto de medición (dispositivo) evaluado.
	MeasuringPointID int64 `json:"measuring_point_id"`
	// WindowHours es la longitud de la ventana: 24 o 6.
	WindowHours int `json:"window_hours"`
	// EvaluatedAt es el fin exclusivo de la ventana (hora en punto).
	EvaluatedAt time.Time `json:"evaluated_at"`
	// Status es CALCULATED o NO_CALCULABLE.
	Status string `json:"status"`
	// NoCalcReason explica un NO_CALCULABLE; vacío si Status = CALCULATED.
	NoCalcReason string `json:"no_calc_reason"`
	// I es el índice de la ventana; nil si no se calculó.
	I *float64 `json:"i"`
	// LowConfidence indica que se evaluó con una línea base de confianza baja.
	LowConfidence bool `json:"low_confidence"`
	// Trigger es quién originó la evaluación (RECONCILER, MANUAL, EVENT).
	Trigger string `json:"trigger"`
	// OccurredAt es el instante en que se generó el evento.
	OccurredAt time.Time `json:"occurred_at"`
	// SchemaVersion es la versión del esquema del evento (ver SchemaVersion).
	SchemaVersion int `json:"schema_version"`
}
