package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// BaselineRepository persiste y consulta las versiones de línea base.
type BaselineRepository interface {
	// GetEffective devuelve la versión con valid_from <= at < valid_to (nil si no hay).
	GetEffective(ctx context.Context, pointID int64, at time.Time) (*entities.BaselineVersion, error)

	// GetLatest devuelve la versión más reciente del punto (nil si no hay).
	GetLatest(ctx context.Context, pointID int64) (*entities.BaselineVersion, error)

	// SaveNewVersion, en una TX: bloquea la vigente, le cierra valid_to = v.ValidFrom,
	// inserta v con version = anterior + 1 y sus estadísticas. Si otro proceso creó
	// la vigente devuelve domain.ErrBaselineConcurrentUpdate.
	SaveNewVersion(ctx context.Context, v entities.BaselineVersion) (entities.BaselineVersion, error)
}
