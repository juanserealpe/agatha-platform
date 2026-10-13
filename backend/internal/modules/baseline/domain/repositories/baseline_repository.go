package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// BaselineRepository persiste y consulta las versiones de línea base.
type BaselineRepository interface {
	// GetLatest devuelve la versión más reciente del punto (nil si no hay).
	GetLatest(ctx context.Context, pointID int64) (*entities.BaselineVersion, error)

	// SaveNewVersion, en una TX: bloquea la vigente, le cierra valid_to = v.ValidFrom,
	// inserta v con version = anterior + 1 y sus estadísticas. Si otro proceso creó
	// la vigente devuelve domain.ErrBaselineConcurrentUpdate.
	SaveNewVersion(ctx context.Context, v entities.BaselineVersion) (entities.BaselineVersion, error)

	// SaveHistory guarda, en UNA sola TX, el histórico retroactivo de un punto
	// SIN versiones: versions debe venir en orden ascendente de ValidFrom; cada
	// una cierra a la anterior y la última queda vigente. Si el punto ya tiene
	// alguna versión devuelve domain.ErrBaselineHistoryExists y no escribe nada.
	SaveHistory(ctx context.Context, pointID int64, versions []entities.BaselineVersion) ([]entities.BaselineVersion, error)
}
