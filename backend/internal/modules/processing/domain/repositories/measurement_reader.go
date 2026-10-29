package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// MeasurementReader lee (solo lectura) las tablas que escribe Ingestion.
// Duplica a propósito el lector de Baseline: cada módulo tiene su propio
// modelo de lectura.
type MeasurementReader interface {
	// GetSamples devuelve los samples B1/B2/Planitud de [from, to), pivotados
	// por timestamp y ordenados (una consulta). Valores no finitos ⇒ Has* = false.
	GetSamples(ctx context.Context, pointID int64, from, to time.Time) ([]features.Sample, error)

	// GetFirstTimestamp devuelve MIN(timestamp) de B2 del punto (nil si no hay).
	GetFirstTimestamp(ctx context.Context, pointID int64) (*time.Time, error)

	// GetLatestTimestamp devuelve MAX(timestamp) de B2 del punto (nil si no hay).
	GetLatestTimestamp(ctx context.Context, pointID int64) (*time.Time, error)

	// ListActiveMeasuringPoints devuelve los puntos activos (tabla devices).
	ListActiveMeasuringPoints(ctx context.Context) ([]int64, error)
}
