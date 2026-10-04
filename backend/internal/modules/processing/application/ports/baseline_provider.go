// Package ports define lo que Processing necesita del exterior y no es una
// tabla propia: la línea base (módulo Baseline, vía adapter en bootstrap) y el reloj.
package ports

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// BaselineProvider entrega la línea base vigente de un punto en una hora.
type BaselineProvider interface {
	// GetEffective devuelve la versión vigente en at. Ausencia de línea base
	// NO es error: devuelve BaselineSnapshot{Available: false, FirstSeenAt: ...}.
	GetEffective(ctx context.Context, pointID int64, at time.Time) (entities.BaselineSnapshot, error)
}
