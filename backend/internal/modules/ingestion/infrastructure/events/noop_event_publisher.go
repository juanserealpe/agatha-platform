// Package events adapta el port EventPublisher de Ingestion a shared/eventbus.
// Es, junto al composition root, el único lugar del módulo que conoce el bus.
package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
)

// NoopEventPublisher descarta los eventos. Se usa con EVENTBUS_ENABLED=false.
type NoopEventPublisher struct{}

// NewNoopEventPublisher crea un publicador que no hace nada.
func NewNoopEventPublisher() *NoopEventPublisher {
	return &NoopEventPublisher{}
}

// PublishMeasurementsIngested no hace nada y nunca falla.
func (p *NoopEventPublisher) PublishMeasurementsIngested(context.Context, contract.MeasurementsIngested) error {
	return nil
}
