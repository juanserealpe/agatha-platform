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

	// ReplaceRange, en UNA TX, borra las filas del punto (ambas ventanas) con
	// evaluated_at en [from, to] (from = nil ⇒ sin límite inferior) e inserta
	// evals. Devuelve cuántas filas borró e insertó. Solo para Rebuild.
	ReplaceRange(ctx context.Context, pointID int64, from *time.Time, to time.Time,
		evals []entities.FeatureEvaluation) (deleted, inserted int, err error)
}
