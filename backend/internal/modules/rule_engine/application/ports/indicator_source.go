package ports

import (
	"context"
	"time"
)

// CalculatedIndicator es una hora CALCULATED de la ventana de 24 h leída de
// feature_evaluations (la fuente de verdad de Processing).
type CalculatedIndicator struct {
	MeasuringPointID int64
	EvaluatedAt      time.Time
	I                float64
	LowConfidence    bool
}

// IndicatorSource lee (solo lectura) feature_evaluations con un modelo propio.
// Lo usa el backfill; el Rule Engine nunca escribe esa tabla.
type IndicatorSource interface {
	// ListPoints devuelve, ascendentes, los puntos con alguna hora CALCULATED de
	// 24 h en [from, to) (cero = sin límite).
	ListPoints(ctx context.Context, from, to time.Time) ([]int64, error)

	// ListCalculated devuelve hasta limit horas CALCULATED de 24 h del punto con
	// evaluated_at > after (cero = desde from) y en [from, to), ascendentes.
	ListCalculated(ctx context.Context, pointID int64, from, to, after time.Time, limit int) ([]CalculatedIndicator, error)
}
