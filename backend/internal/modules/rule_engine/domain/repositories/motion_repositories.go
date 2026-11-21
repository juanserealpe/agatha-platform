package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// MotionThresholdRepository lee motion_threshold_versions. Las escrituras solo
// ocurren dentro de Transaction (cerrar la vigente e insertar la nueva van
// juntas), con las mismas reglas que rule_threshold_versions. Un conflicto de
// concurrencia se devuelve envolviendo ErrConflict.
type MotionThresholdRepository interface {
	// GetEffectiveAt devuelve la versión que aplica a la hora t; nil si ninguna.
	GetEffectiveAt(ctx context.Context, pointID int64, t time.Time) (*entities.MotionThresholdVersion, error)

	// GetHistory devuelve todas las versiones del punto, ascendentes por versión.
	GetHistory(ctx context.Context, pointID int64) ([]entities.MotionThresholdVersion, error)

	// ListPoints devuelve, ascendentes, los puntos que tienen al menos una versión.
	ListPoints(ctx context.Context) ([]int64, error)

	Transaction(ctx context.Context, fn func(tx MotionThresholdWriter) error) error
}

// MotionThresholdWriter es el acceso de escritura ligado a una transacción.
type MotionThresholdWriter interface {
	// LockOpen devuelve la versión abierta (valid_to IS NULL) con FOR UPDATE
	// NOWAIT; nil si el punto no tiene.
	LockOpen(ctx context.Context, pointID int64) (*entities.MotionThresholdVersion, error)

	// Close cierra una versión: valid_to = validTo (única modificación permitida).
	Close(ctx context.Context, versionID int64, validTo time.Time) error

	// Insert crea la versión y devuelve su ID.
	Insert(ctx context.Context, v entities.MotionThresholdVersion) (int64, error)
}

// MotionEvaluationRepository persiste motion_evaluations: una fila por punto y
// hora, también NO_CALCULABLE. La hora siguiente lee el estado de la anterior.
type MotionEvaluationRepository interface {
	// GetLastBefore devuelve la última fila del punto con evaluated_at < before
	// (cero = sin límite); nil si no tiene.
	GetLastBefore(ctx context.Context, pointID int64, before time.Time) (*entities.MotionEvaluation, error)

	// Save inserta la fila con ON CONFLICT (punto, hora) DO NOTHING.
	// inserted = false ⇒ otro proceso ya la había guardado.
	Save(ctx context.Context, e entities.MotionEvaluation) (id int64, inserted bool, err error)

	// DeleteRange borra las filas de [from, to) (cero = sin límite) del punto.
	// Solo lo usa el backfill con -rebuild.
	DeleteRange(ctx context.Context, pointID int64, from, to time.Time) (int64, error)

	// ListEpisodes devuelve los episodios cerrados del punto (horas CLEARED
	// con evaluated_at < before; cero = sin límite), ascendentes. Los usa el
	// historial de motion-v2.
	ListEpisodes(ctx context.Context, pointID int64, before time.Time) ([]entities.MotionEpisode, error)

	// ListRange devuelve las filas del punto con from <= evaluated_at <= to, ascendentes.
	ListRange(ctx context.Context, pointID int64, from, to time.Time) ([]entities.MotionEvaluation, error)

	// CountAfter cuenta las filas del punto con evaluated_at >= at.
	CountAfter(ctx context.Context, pointID int64, at time.Time) (int64, error)
}
