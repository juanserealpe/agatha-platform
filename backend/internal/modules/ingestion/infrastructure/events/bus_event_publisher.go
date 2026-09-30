package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
)

// BusEventPublisher publica los eventos de Ingestion en shared/eventbus.
type BusEventPublisher struct {
	bus eventbus.Bus
}

// NewBusEventPublisher crea el publicador sobre el bus recibido.
func NewBusEventPublisher(bus eventbus.Bus) *BusEventPublisher {
	return &BusEventPublisher{bus: bus}
}

// PublishMeasurementsIngested publica en TopicMeasurementsIngested. El bus no
// bloquea ni falla, por lo que siempre devuelve nil.
func (p *BusEventPublisher) PublishMeasurementsIngested(ctx context.Context, event contract.MeasurementsIngested) error {
	eventbus.PublishTyped(ctx, p.bus, contract.TopicMeasurementsIngested, event)
	return nil
}
