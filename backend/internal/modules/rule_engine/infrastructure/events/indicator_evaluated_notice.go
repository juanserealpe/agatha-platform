// Package events recibe los eventos que le interesan al Rule Engine. No conoce
// el bus ni a los publicadores: internal/bootstrap traduce y suscribe.
package events

import "time"

// IndicatorEvaluatedNotice es la vista propia del Rule Engine de una ventana
// (24 h o 6 h) evaluada por Processing. Es una pista: la fila de
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
