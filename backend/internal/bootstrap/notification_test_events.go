package bootstrap

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	notificationPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// NotificationsTestEndpointEnv habilita POST /api/notifications/test-event
// (solo si AGATHA_ENV no es production). Nunca en el VPS: el evento llega a
// los canales reales, incluido el push a los celulares registrados.
const NotificationsTestEndpointEnv = "NOTIFICATIONS_TEST_ENDPOINT"

// testEventSource marca los eventos de prueba en el campo Source del contrato.
const testEventSource = "TEST"

// testEndpointEnabled decide si se registra el endpoint de eventos de prueba.
func testEndpointEnabled(log *logger.Logger) bool {
	if !envBool(log, NotificationsTestEndpointEnv, false) {
		return false
	}
	if os.Getenv(EnvironmentEnv) == "production" {
		log.Error("endpoint de eventos de prueba NO habilitado: AGATHA_ENV=production", "env", NotificationsTestEndpointEnv)
		return false
	}
	log.Warn("endpoint de eventos de prueba habilitado: POST /api/notifications/test-event (no usar en el VPS)")
	return true
}

// busTestEventEmitter implementa ports.TestEventEmitter publicando los mismos
// contratos que Alert Management. Así el evento recorre el pipeline real
// (suscripciones, políticas, hub SSE y push) sin escribir en la base.
type busTestEventEmitter struct {
	bus   eventbus.Bus
	clock notificationPorts.Clock
	// seq numera alertas y mensajes de prueba con ids negativos: nunca chocan
	// con los reales (que son positivos) en la caché del frontend.
	seq atomic.Int64
}

func newBusTestEventEmitter(bus eventbus.Bus, clock notificationPorts.Clock) *busTestEventEmitter {
	return &busTestEventEmitter{bus: bus, clock: clock}
}

func testAlertCode(alertID int64) string {
	return fmt.Sprintf("ALR-PRUEBA-%06d", -alertID)
}

func (e *busTestEventEmitter) Emit(ctx context.Context, ev notificationEntities.TestEvent) (notificationPorts.TestEventResult, error) {
	now := e.clock.Now().UTC()
	evaluated := now.Truncate(time.Hour)
	alertID := ev.AlertID
	if alertID == 0 {
		alertID = -e.seq.Add(1)
	}
	result := notificationPorts.TestEventResult{AlertID: alertID, AlertCode: testAlertCode(alertID), Change: ev.ChangeKind()}

	state := "ACTIVA"
	if result.Change == notificationEntities.ChangeFinished {
		state = "FINALIZADA"
	}
	status := "SIN_CLASIFICAR"
	switch {
	case result.Change == notificationEntities.ChangeLabeled:
		status = "CLASIFICADA_CON_ETIQUETA"
	case ev.Classification != "":
		status = "CLASIFICADA_SIN_ETIQUETA"
	}

	eventbus.PublishTyped(ctx, e.bus, alertContract.TopicAlertChanged, alertContract.AlertChanged{
		AlertID: alertID, AlertCode: result.AlertCode, MeasuringPointID: ev.MeasuringPointID, EventType: ev.EventType,
		Change: result.Change, State: state, CurrentLevel: ev.Level, MaxLevel: ev.Level,
		Blinking:         state == "ACTIVA" && ev.Level == notificationEntities.LevelRojo && ev.Classification == "",
		ManagementStatus: status, Classification: ev.Classification,
		StartedAt: evaluated, EvaluatedAt: evaluated, OccurredAt: now, SchemaVersion: alertContract.SchemaVersion,
	})

	if ev.Notifies() {
		result.MessageID = -e.seq.Add(1)
		eventbus.PublishTyped(ctx, e.bus, alertContract.TopicAlertNotification, alertContract.AlertNotification{
			MessageID: result.MessageID, AlertID: alertID, AlertCode: result.AlertCode,
			MeasuringPointID: ev.MeasuringPointID, EventType: ev.EventType,
			StartedAt: evaluated, EvaluatedAt: evaluated, Code: ev.Code, Level: ev.Level, MaxLevel: ev.Level,
			AlertState: state, Classification: ev.Classification,
			Title:  fmt.Sprintf("[PRUEBA] %s: nivel %s", result.AlertCode, ev.Level),
			Body:   "Evento de prueba: no corresponde a una alerta real.",
			Source: testEventSource, Late: ev.Late, OccurredAt: now, SchemaVersion: alertContract.SchemaVersion,
		})
	}
	return result, nil
}
