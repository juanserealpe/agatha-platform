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

	// Save crea un nuevo estado de ingesta.
	Save(
		ctx context.Context,
		state *entities.IngestionState,
	) error

	// Update actualiza el estado de ingesta existente.
	Update(
		ctx context.Context,
		state *entities.IngestionState,
	) error
}
