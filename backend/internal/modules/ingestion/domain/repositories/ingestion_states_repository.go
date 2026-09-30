package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type IngestionStateRepository interface {

	// FindLatestByMeasuringPointID busca el último estado de ingesta para un punto de medición específico.
	FindLatestByMeasuringPointID(
		ctx context.Context,
		measuringPointID int64,
	) (*entities.IngestionState, error)

	// Save crea un nuevo estado de ingesta (una fila por lote) y devuelve el
	// cursor del lote (id de la fila) y el last_ingested_at persistido.
	Save(
		ctx context.Context,
		state *entities.IngestionState,
	) (entities.SaveStateResult, error)

	// Update actualiza el estado de ingesta existente.
	Update(
		ctx context.Context,
		state *entities.IngestionState,
	) error
}
