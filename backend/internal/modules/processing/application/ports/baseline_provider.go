// Package ports define lo que Processing necesita del exterior y no es una
// tabla propia: la línea base (tablas de Baseline, con un modelo de lectura
// propio) y el reloj.
package ports

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// BaselineProvider entrega las versiones de línea base de un punto. Processing
// solo las LEE: quien las escribe es el módulo Baseline.
type BaselineProvider interface {
	// GetVersions devuelve, en orden ascendente de ValidFrom, las versiones
	// vigentes en algún instante de [from, to]. Sin versiones ⇒ lista vacía
	// (no es error).
	GetVersions(ctx context.Context, pointID int64, from, to time.Time) ([]entities.BaselineSnapshot, error)

	// GetFirstValidFrom devuelve el valid_from de la primera versión del punto
	// (nil si todavía no tiene ninguna).
	GetFirstValidFrom(ctx context.Context, pointID int64) (*time.Time, error)
}
