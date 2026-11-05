package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
)

// NoopEventPublisher descarta los eventos (procesos sin consumidores).
type NoopEventPublisher struct{}

// NewNoopEventPublisher crea un publicador que no hace nada.
func NewNoopEventPublisher() *NoopEventPublisher {
	return &NoopEventPublisher{}
}

// PublishIndicatorEvaluated no hace nada y nunca falla.
func (p *NoopEventPublisher) PublishIndicatorEvaluated(context.Context, contract.IndicatorEvaluated) error {
	return nil
}
