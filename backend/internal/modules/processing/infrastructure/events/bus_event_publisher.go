package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
)

// BusEventPublisher publica los eventos de Processing en shared/eventbus. Es,
// junto al composition root, el único lugar del módulo que conoce el bus.
type BusEventPublisher struct {
	bus eventbus.Bus
}

// NewBusEventPublisher crea el publicador sobre el bus recibido.
func NewBusEventPublisher(bus eventbus.Bus) *BusEventPublisher {
	return &BusEventPublisher{bus: bus}
}

// PublishIndicatorEvaluated publica en TopicIndicatorEvaluated. El bus no
// bloquea ni falla, por lo que siempre devuelve nil.
func (p *BusEventPublisher) PublishIndicatorEvaluated(ctx context.Context, event contract.IndicatorEvaluated) error {
	eventbus.PublishTyped(ctx, p.bus, contract.TopicIndicatorEvaluated, event)
	return nil
}
