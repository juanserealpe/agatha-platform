package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// HistoryReader lee (solo lectura) el histórico que escribe Ingestion.
type HistoryReader interface {
	// GetSamples devuelve los samples (B1, B2, Planitud) del punto en [from, to),
	// pivotados por timestamp y ordenados. Valores no finitos ⇒ Has* = false.
	GetSamples(ctx context.Context, pointID int64, from, to time.Time) ([]features.Sample, error)

	// GetFirstSeenAt devuelve el primer timestamp con B2 del punto (nil si no hay).
	GetFirstSeenAt(ctx context.Context, pointID int64) (*time.Time, error)

	// ListActiveMeasuringPoints devuelve los puntos de medición activos.
	ListActiveMeasuringPoints(ctx context.Context) ([]int64, error)
}
