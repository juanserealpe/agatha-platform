package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

type MeasurementRepository interface {
	// GetLatestMeasurements devuelve las mediciones del punto de medición en su
	// timestamp más reciente. Devuelve un slice vacío si el punto no tiene mediciones.
	GetLatestMeasurements(ctx context.Context, measuringPointID int64,
	) ([]entities.RawMeasurement, error)
}
