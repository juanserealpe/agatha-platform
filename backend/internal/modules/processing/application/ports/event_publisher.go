package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
)

// EventPublisher publica los eventos de integración de Processing.
//
// Se invoca SIEMPRE después de persistir; un error nunca hace fallar la evaluación.
type EventPublisher interface {
	PublishIndicatorEvaluated(ctx context.Context, event contract.IndicatorEvaluated) error
}
