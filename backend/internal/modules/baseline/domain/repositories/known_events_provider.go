package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// KnownEventsProvider entrega los eventos conocidos de un punto (fugas,
// intervenciones, reparaciones) cuyos intervalos no deben usarse para calcular
// la línea base.
//
// Hoy lo implementa una lista estática en código
// (infrastructure/knownevents). Cuando exista el módulo de alertas, su
// adapter en internal/bootstrap reemplaza esa implementación sin tocar el
// cálculo.
type KnownEventsProvider interface {
	// GetKnownEvents devuelve los eventos del punto que se solapan con
	// [from, to), en UTC y ordenados por From. Sin eventos ⇒ lista vacía.
	GetKnownEvents(ctx context.Context, pointID int64, from, to time.Time) ([]entities.KnownEvent, error)
}
