package bootstrap

import (
	"context"
	"os"
	"strings"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	notificationServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	notificationEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/fcm"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const (
	// alertNotificationSubscription es el nombre de la suscripción de Notifications.
	alertNotificationSubscription = "notifications.alert_notification"

	// PushTopicEnv es el topic de FCM al que se suscriben las apps móviles.
	PushTopicEnv     = "NOTIFICATIONS_PUSH_TOPIC"
	DefaultPushTopic = "alerts"
)

// NotificationsModule expone el módulo de notificaciones ya cableado.
type NotificationsModule struct {
	subscriber *notificationEvents.AlertNotificationSubscriber
	bus        eventbus.Bus
	log        *logger.Logger
}

// NewNotificationsModule crea el cliente de FCM (GOOGLE_APPLICATION_CREDENTIALS)
// y el caso de uso. No usa la BD ni importa a Alert Management: la traducción
// del evento vive acá.
func NewNotificationsModule(platform *Platform) (*NotificationsModule, error) {
	log := logger.New("Notifications", "Info")

	sender, err := fcm.NewSender(context.Background())
	if err != nil {
		log.Error("no se pudo crear el cliente de FCM", "error", err)
		return nil, err
	}
	topic := PushTopic()
	log.Info("push de alertas por FCM", "topic", topic)

	return &NotificationsModule{
		subscriber: notificationEvents.NewAlertNotificationSubscriber(
			notificationServices.NewAlertPushService(sender, topic, log), log),
		bus: platform.EventBus,
		log: log,
	}, nil
}

// PushTopic devuelve NOTIFICATIONS_PUSH_TOPIC o, si está vacía, DefaultPushTopic.
func PushTopic() string {
	if topic := strings.TrimSpace(os.Getenv(PushTopicEnv)); topic != "" {
		return topic
	}
	return DefaultPushTopic
}

// SubscribeAlertNotification suscribe el módulo a alert_management.alert_notification.
// Debe llamarse antes de arrancar los schedulers.
func (m *NotificationsModule) SubscribeAlertNotification() (unsubscribe func()) {
	return subscribeAlertNotification(m.bus, m.subscriber, m.log)
}

// alertNoticeHandler es lo que Notifications hace con cada aviso.
type alertNoticeHandler interface {
	Handle(ctx context.Context, n notificationEntities.AlertNotice)
}

// subscribeAlertNotification traduce el contrato de Alert Management al
// AlertNotice de Notifications (que no importa a Alert Management).
func subscribeAlertNotification(bus eventbus.Bus, handler alertNoticeHandler, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, alertContract.TopicAlertNotification,
		func(ctx context.Context, e alertContract.AlertNotification) {
			handler.Handle(ctx, toAlertNotice(e))
		},
		log,
		eventbus.WithName(alertNotificationSubscription),
	)
}

func toAlertNotice(e alertContract.AlertNotification) notificationEntities.AlertNotice {
	return notificationEntities.AlertNotice{
		MessageID:        e.MessageID,
		AlertID:          e.AlertID,
		AlertCode:        e.AlertCode,
		MeasuringPointID: e.MeasuringPointID,
		EventType:        e.EventType,
		EvaluatedAt:      e.EvaluatedAt.UTC(),
		Code:             e.Code,
		Level:            e.Level,
		MaxLevel:         e.MaxLevel,
		AlertState:       e.AlertState,
		Title:            e.Title,
		Body:             e.Body,
		Source:           e.Source,
		Late:             e.Late,
	}
}
