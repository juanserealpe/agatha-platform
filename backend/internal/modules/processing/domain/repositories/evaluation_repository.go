package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// EvaluationRepository persiste las evaluaciones (tabla feature_evaluations).
type EvaluationRepository interface {
	// GetLastEvaluatedAt devuelve la última hora evaluada del punto en la
	// ventana de 24 h (las de 6 h se escriben en la misma TX). nil si no hay.
	GetLastEvaluatedAt(ctx context.Context, pointID int64) (*time.Time, error)

	// SaveAll inserta en una TX con ON CONFLICT (punto, ventana, hora) DO
	// NOTHING. inserted < len(evals) significa que otro proceso ya las insertó.
	SaveAll(ctx context.Context, evals []entities.FeatureEvaluation) (inserted int, err error)
}
