package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// KnownEventsProvider entrega los períodos de exclusión de un punto, que no
// deben usarse para calcular la línea base.
//
// Lo implementa ExclusionPeriodRepository (baseline_exclusion_periods: copia de
// todas las alertas más los períodos manuales; ningún rebuild la borra). El
// margen lo aplica el dominio, no el provider.
type KnownEventsProvider interface {
	// GetKnownEvents devuelve los eventos del punto que se solapan con
	// [from, to), en UTC y ordenados por From. Sin eventos ⇒ lista vacía.
	GetKnownEvents(ctx context.Context, pointID int64, from, to time.Time) ([]entities.KnownEvent, error)
}
