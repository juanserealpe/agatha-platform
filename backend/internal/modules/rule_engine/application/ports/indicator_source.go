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
	// DPl es el % de registros con planitud alta (nil si no se calculó).
	DPl           *float64
	LowConfidence bool
}

// IndicatorSource lee (solo lectura) feature_evaluations con un modelo propio.
// Lo usan el backfill, la simulación y, para el d_pl, la evaluación en vivo;
// el Rule Engine nunca escribe esa tabla.
type IndicatorSource interface {
	// ListPoints devuelve, ascendentes, los puntos con alguna hora CALCULATED de
	// 24 h en [from, to) (cero = sin límite).
	ListPoints(ctx context.Context, from, to time.Time) ([]int64, error)

	// ListCalculated devuelve hasta limit horas CALCULATED de 24 h del punto con
	// evaluated_at > after (cero = desde from) y en [from, to), ascendentes.
	ListCalculated(ctx context.Context, pointID int64, from, to, after time.Time, limit int) ([]CalculatedIndicator, error)

	// GetDPl devuelve el d_pl de la hora de 24 h del punto (nil si la fila no
	// existe o no lo tiene). El evento de Processing no lo lleva.
	GetDPl(ctx context.Context, pointID int64, at time.Time) (*float64, error)
}
