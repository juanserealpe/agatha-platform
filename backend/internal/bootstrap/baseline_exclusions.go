package bootstrap

import (
	"context"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	baselineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// baselineExclusionsSubscription es el nombre de la suscripción de Baseline a
// alert_management.alert_changed.
const baselineExclusionsSubscription = "baseline.alert_changed.exclusions"

// alertSnapshotRecorder es lo que la suscripción necesita de Baseline
// (ExclusionService.RecordAlert).
type alertSnapshotRecorder interface {
	RecordAlert(ctx context.Context, a baselineServices.AlertSnapshot) error
}

// subscribeBaselineExclusions copia a baseline_exclusion_periods el intervalo
// de cada foto de alerta (OPENED, FINISHED, REACTIVATED y cualquier otro
// cambio: el upsert por código es idempotente). Baseline no conoce el bus ni
// alert_management: la traducción vive acá. Un fallo se registra en el
// servicio y no afecta a Alert Management (se publica después del commit).
func subscribeBaselineExclusions(bus eventbus.Bus, recorder alertSnapshotRecorder, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, alertContract.TopicAlertChanged,
		func(ctx context.Context, c alertContract.AlertChanged) {
			_ = recorder.RecordAlert(ctx, toAlertSnapshot(c))
		},
		log,
		eventbus.WithName(baselineExclusionsSubscription),
	)
}

func toAlertSnapshot(c alertContract.AlertChanged) baselineServices.AlertSnapshot {
	return baselineServices.AlertSnapshot{
		AlertID:          c.AlertID,
		AlertCode:        c.AlertCode,
		MeasuringPointID: c.MeasuringPointID,
		StartedAt:        c.StartedAt,
		EndedAt:          c.EndedAt,
	}
}
