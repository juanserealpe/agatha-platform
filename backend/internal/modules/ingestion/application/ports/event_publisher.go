package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
)

// EventPublisher publica los eventos de integración de Ingestion.
//
// Se invoca SIEMPRE después del commit; un error nunca hace fallar la ingesta.
type EventPublisher interface {
	PublishMeasurementsIngested(ctx context.Context, event contract.MeasurementsIngested) error
}
