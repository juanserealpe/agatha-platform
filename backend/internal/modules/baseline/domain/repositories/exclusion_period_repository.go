package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// ExclusionPeriodRepository persiste los períodos de exclusión
// (baseline_exclusion_periods). La lectura para el cálculo es
// KnownEventsProvider, que implementa el mismo adaptador.
type ExclusionPeriodRepository interface {
	// UpsertAlert inserta el período de una alerta o, si ya existe uno con su
	// SourceAlertCode, actualiza punto, inicio y fin. Idempotente.
	UpsertAlert(ctx context.Context, p entities.ExclusionPeriod) error
	// CountOpenAlerts cuenta los períodos ALERT del punto sin fin (alertas
	// vigentes). Con alguno, -rebuild se rechaza: Alert Management recrearía
	// la alerta con otro código y su período quedaría abierto para siempre.
	CountOpenAlerts(ctx context.Context, pointID int64) (int, error)
}

// AlertIntervalReader lee los intervalos de las alertas actuales (tabla
// alerts de Alert Management, modelo propio, solo lectura) para el seed.
type AlertIntervalReader interface {
	// ListAlertIntervals devuelve un período (Source ALERT) por alerta, con
	// SourceAlertCode = código de la alerta.
	ListAlertIntervals(ctx context.Context) ([]entities.ExclusionPeriod, error)
}
