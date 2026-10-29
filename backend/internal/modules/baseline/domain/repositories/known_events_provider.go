package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// KnownEventsProvider entrega los intervalos de las alertas de un punto, que no
// deben usarse para calcular la línea base.
//
// Lo implementa AlertEventsReader (todas las alertas del punto, leídas de
// alerts con modelo propio). El margen lo aplica el dominio, no el provider.
type KnownEventsProvider interface {
	// GetKnownEvents devuelve los eventos del punto que se solapan con
	// [from, to), en UTC y ordenados por From. Sin eventos ⇒ lista vacía.
	GetKnownEvents(ctx context.Context, pointID int64, from, to time.Time) ([]entities.KnownEvent, error)
}
