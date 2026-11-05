package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
)

// IngestionUnitOfWork ejecuta fn en una transacción: si fn devuelve error, se
// revierte todo lo escrito con los repositorios recibidos.
type IngestionUnitOfWork interface {
	Execute(
		ctx context.Context,
		fn func(
			ctx context.Context,
			measurementRepository repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
		) error,
	) error
}
