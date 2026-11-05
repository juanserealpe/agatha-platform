package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
)

// SeriesReader lee los puntos de las variables de UNA fuente
// (entities.SeriesSource). Hay una implementación por fuente; agregar una
// fuente nueva es agregar un lector y registrarlo en bootstrap.
type SeriesReader interface {
	// Read devuelve, por clave de variable, sus puntos en [startDate, endDate)
	// ordenados por tiempo. Una variable sin datos puede faltar en el mapa.
	Read(
		ctx context.Context,
		deviceID int64,
		definitions []entities.SeriesDefinition,
		startDate time.Time,
		endDate time.Time,
	) (map[string][]entities.SeriesPoint, error)
}
