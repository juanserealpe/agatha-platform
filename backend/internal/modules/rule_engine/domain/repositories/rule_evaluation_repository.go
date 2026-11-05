package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// RuleEvaluationRepository persiste rule_evaluations (una fila por punto,
// evento y hora calculada).
type RuleEvaluationRepository interface {
	// GetPosition indica si ya existe la fila de la hora at (exists) y si hay
	// filas posteriores del punto (hasLater: llegada tardía o desordenada).
	GetPosition(ctx context.Context, pointID int64, eventType entities.EventType, at time.Time) (exists, hasLater bool, err error)

	// GetPrevious devuelve hasta limit filas con evaluated_at < before, de la
	// más reciente a la más antigua.
	GetPrevious(ctx context.Context, pointID int64, eventType entities.EventType, before time.Time, limit int) ([]entities.RuleEvaluation, error)

	// Save inserta la fila en una transacción con ON CONFLICT (punto, evento,
	// hora) DO NOTHING. inserted = false ⇒ otro proceso ya la había guardado.
	// Devuelve el ID asignado cuando se insertó.
	Save(ctx context.Context, e entities.RuleEvaluation) (id int64, inserted bool, err error)

	// DeleteRange borra las filas del rango [from, to) (cero = sin límite) del
	// punto (nil = todos). Solo lo usa el backfill con -rebuild.
	DeleteRange(ctx context.Context, pointID *int64, eventType entities.EventType, from, to time.Time) (int64, error)
}
