package repositories

import (
	"context"
	"time"
)

// MeasurementCoverageRepository responde qué instantes ya tienen mediciones,
// sin importar la vía que las trajo (callback o DataOS). Lo usa el catch-up
// para encontrar huecos.
type MeasurementCoverageRepository interface {
	// LatestTimestamp devuelve el último timestamp del punto, o nil si no tiene mediciones.
	LatestTimestamp(ctx context.Context, measuringPointID int64) (*time.Time, error)

	// ListTimestamps devuelve los timestamps distintos del punto en [from, to], ascendentes.
	ListTimestamps(ctx context.Context, measuringPointID int64, from, to time.Time) ([]time.Time, error)
}
