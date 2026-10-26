package knownevents

import (
	"context"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
)

// CompositeProvider une varias fuentes de eventos conocidos (la lista estática
// y las alertas). Si una falla, falla todo: una línea base
// calculada sin parte de los eventos estaría contaminada.
type CompositeProvider struct {
	providers []repositories.KnownEventsProvider
}

func NewCompositeProvider(providers ...repositories.KnownEventsProvider) *CompositeProvider {
	return &CompositeProvider{providers: providers}
}

// GetKnownEvents devuelve los eventos de todas las fuentes, ordenados por From.
// Los solapes no se fusionan: la exclusión ya trata cada intervalo por separado.
func (p *CompositeProvider) GetKnownEvents(ctx context.Context, pointID int64, from, to time.Time) ([]entities.KnownEvent, error) {
	out := []entities.KnownEvent{}
	for _, provider := range p.providers {
		events, err := provider.GetKnownEvents(ctx, pointID, from, to)
		if err != nil {
			return nil, err
		}
		out = append(out, events...)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].From.Before(out[b].From) })
	return out, nil
}
