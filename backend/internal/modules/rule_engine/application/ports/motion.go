package ports

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// Métricas de raw_measurements que lee el movimiento (catálogo fijo 1..21).
const (
	MetricPitch       = 1
	MetricRoll        = 2
	MetricDeriva      = 19
	MetricTemperature = 20
)

// RawMeasurementSource lee (solo lectura, modelo propio) las métricas 1, 2, 19
// y 20 de raw_measurements. La tabla es de Ingestion: el Rule Engine nunca la escribe.
type RawMeasurementSource interface {
	// ListRecords devuelve los registros del punto en [from, to), agrupados por
	// timestamp y ascendentes.
	ListRecords(ctx context.Context, pointID int64, from, to time.Time) ([]entities.RawRecord, error)

	// FirstMeasurementAt es la primera medición de pitch o roll del punto (nil si no tiene).
	FirstMeasurementAt(ctx context.Context, pointID int64) (*time.Time, error)

	// ListPoints devuelve, ascendentes, los puntos de devices (la tabla de Ingestion).
	ListPoints(ctx context.Context) ([]int64, error)
}

// MotionEventPublisher publica rule_engine.motion_evaluated. Se invoca después
// del commit; un error nunca hace fallar la evaluación. El adaptador al bus
// vive en internal/bootstrap.
type MotionEventPublisher interface {
	PublishMotionEvaluated(ctx context.Context, event contract.MotionEvaluated) error
}
