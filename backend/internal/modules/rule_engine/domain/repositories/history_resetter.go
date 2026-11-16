package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// HistoryResetter reconstruye el historial de versiones de umbrales de UN
// punto. Es la segunda excepción deliberada a la no retroactividad (la primera
// es la v1 sembrada): solo la usa cmd/rule_engine -backfill -rebuild
// -reset-versions, acotada a un punto y bloqueada en production.
type HistoryResetter interface {
	// Transaction ejecuta fn en UNA transacción: si fn devuelve error no queda nada escrito.
	Transaction(ctx context.Context, fn func(tx HistoryResetTx) error) error
}

// HistoryResetTx son las operaciones del reinicio, ligadas a una transacción.
// El orden que importa: rule_evaluations referencia la versión por FK
// (RESTRICT), así que las filas se borran ANTES que las versiones.
type HistoryResetTx interface {
	// LockVersions devuelve todas las versiones del punto y evento, ascendentes
	// por versión, bloqueándolas hasta el fin de la transacción
	// (FOR UPDATE NOWAIT: si otra transacción las tiene, falla con ErrConflict).
	LockVersions(ctx context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error)

	// FirstMeasurementAt es la primera medición del punto en raw_measurements
	// (nil si no tiene ninguna).
	FirstMeasurementAt(ctx context.Context, pointID int64) (*time.Time, error)

	// DeleteEvaluations borra TODAS las filas de rule_evaluations del punto y evento.
	DeleteEvaluations(ctx context.Context, pointID int64, eventType entities.EventType) (int64, error)

	// DeleteVersions borra TODAS las versiones del punto y evento.
	DeleteVersions(ctx context.Context, pointID int64, eventType entities.EventType) (int64, error)

	// InsertVersion crea la versión y devuelve su ID.
	InsertVersion(ctx context.Context, v entities.ThresholdVersion) (int64, error)
}
