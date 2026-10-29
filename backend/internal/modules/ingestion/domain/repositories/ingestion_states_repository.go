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

	// SaveIfAbsent crea el estado solo si no existe otro con el mismo RecordID
	// (requerido). created = false indica un duplicado ya persistido; en ese caso
	// el resultado viene vacío. Es seguro ante inserciones concurrentes.
	SaveIfAbsent(
		ctx context.Context,
		state *entities.IngestionState,
	) (result entities.SaveStateResult, created bool, err error)

	// Update actualiza el estado de ingesta existente.
	Update(
		ctx context.Context,
		state *entities.IngestionState,
	) error
}
