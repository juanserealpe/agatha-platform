package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type MeasurementRepository interface {
	// SaveBatch persiste el lote y devuelve cuántas filas se insertaron realmente.
	SaveBatch(ctx context.Context, measurements []entities.RawMeasurement) (entities.SaveBatchResult, error)
}
