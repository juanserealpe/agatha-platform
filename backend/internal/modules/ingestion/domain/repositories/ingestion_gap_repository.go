package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type IngestionGapRepository interface {

	// Busca el gap actualmente abierto para un punto de medición.
	FindOpenByMeasuringPointID(
		ctx context.Context,
		measuringPointID int64,
	) (*entities.IngestionGap, error)

	// Crea un nuevo gap.
	Save(
		ctx context.Context,
		gap *entities.IngestionGap,
	) error

	// Actualiza un gap existente.
	Update(
		ctx context.Context,
		gap *entities.IngestionGap,
	) error
}
