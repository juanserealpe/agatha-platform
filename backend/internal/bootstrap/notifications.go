package bootstrap

import (
	"context"
	"net/http"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	notificationPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	notificationServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/policies"
	notificationEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/realtime"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// NotificationsSSEMaxClientsEnv es el tope de streams SSE abiertos a la vez.
const NotificationsSSEMaxClientsEnv = "NOTIFICATIONS_SSE_MAX_CLIENTS"

// Nombres de las suscripciones de Notifications: una por canal, cada una con
// su cola y su worker (si FCM se atrasa, la web no lo nota).
const (
	noticeWebSubscription  = "notifications.alert_notification.web"
	noticePushSubscription = "notifications.alert_notification.push"
	changeWebSubscription  = "notifications.alert_changed.web"
)

// notificationChannels son los canales de entrega. Un canal nil no se suscribe.
type notificationChannels struct {
	webNotices notificationPorts.NoticeChannel
	webChanges notificationPorts.ChangeChannel
	push       notificationPorts.NoticeChannel
}

// NotificationsModule expone Notifications ya cableado.
type NotificationsModule struct {
	channels notificationChannels
	bus      eventbus.Bus
	log      *logger.Logger
	// hub es el canal web (avisos y cambios) y expone el stream SSE.
	hub *realtime.Hub
}

// NewNotificationsModule cablea Notifications. Consume los eventos de Alert
// Management, pero no lo exige: sin él no recibe nada (ver ParseModules).
func NewNotificationsModule(platform *Platform) *NotificationsModule {
	log := logger.New("Notifications", "Info")
	hub := realtime.NewHub(realtime.HubConfig{
		MaxClients: envInt(log, NotificationsSSEMaxClientsEnv, realtime.DefaultMaxClients),
	}, log)
	m := wireNotifications(platform.EventBus, notificationChannels{webNotices: hub, webChanges: hub}, log)
	m.hub = hub
	return m
}

func wireNotifications(bus eventbus.Bus, channels notificationChannels, log *logger.Logger) *NotificationsModule {
	return &NotificationsModule{channels: channels, bus: bus, log: log}
}

// Subscribe suscribe un despachador por canal disponible. Debe llamarse antes
// de arrancar los schedulers.
func (m *NotificationsModule) Subscribe() (unsubscribe func()) {
	var unsubscribes []func()
	if m.channels.webNotices != nil {
		unsubscribes = append(unsubscribes, subscribeAlertNotification(m.bus, noticeWebSubscription,
			notificationServices.NewNoticeDispatcher("web", policies.WebPolicy{}, m.channels.webNotices, m.log), m.log))
	}
	if m.channels.webChanges != nil {
		unsubscribes = append(unsubscribes, subscribeAlertChanged(m.bus, changeWebSubscription,
			notificationEvents.NewAlertChangedSubscriber(m.channels.webChanges, m.log), m.log))
	}
	if m.channels.push != nil {
		unsubscribes = append(unsubscribes, subscribeAlertNotification(m.bus, noticePushSubscription,
			notificationServices.NewNoticeDispatcher("push", policies.MobilePushPolicy{}, m.channels.push, m.log), m.log))
	}
	if len(unsubscribes) == 0 {
		m.log.Warn("Notifications no tiene canales de entrega configurados; los avisos se descartan")
	}
	return func() {
		for _, u := range unsubscribes {
			u()
		}
	}
}

// RegisterRoutes expone GET /api/notifications/stream.
func (m *NotificationsModule) RegisterRoutes(mux *http.ServeMux) {
	if m.hub != nil {
		realtime.NewStreamHandler(m.hub, m.log).RegisterRoutes(mux)
	}
}

// Runners devuelve el heartbeat del hub. Al apagarse cierra los streams
// abiertos, para que el apagado del servidor HTTP no los espere.
func (m *NotificationsModule) Runners() []Runner {
	if m.hub == nil {
		return nil
	}
	return []Runner{{Name: "hub SSE de notificaciones", Start: m.hub.Run}}
}

// subscribeAlertNotification traduce el contrato de Alert Management al Notice
// de Notifications (que no importa a Alert Management). El bus entrega en orden.
func subscribeAlertNotification(bus eventbus.Bus, name string, dispatcher notificationEvents.NoticeDispatcher, log *logger.Logger) (unsubscribe func()) {
	subscriber := notificationEvents.NewAlertNotificationSubscriber(dispatcher)
	return eventbus.SubscribeTyped(bus, alertContract.TopicAlertNotification,
		func(ctx context.Context, n alertContract.AlertNotification) {
			subscriber.Handle(ctx, toAlertNotice(n))
		},
		log,
		eventbus.WithName(name),
	)
}

// subscribeAlertChanged traduce y entrega cada foto al canal web.
func subscribeAlertChanged(bus eventbus.Bus, name string, subscriber *notificationEvents.AlertChangedSubscriber, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, alertContract.TopicAlertChanged,
		func(ctx context.Context, c alertContract.AlertChanged) {
			subscriber.Handle(ctx, toAlertChange(c))
		},
		log,
		eventbus.WithName(name),
	)
}

func toAlertNotice(n alertContract.AlertNotification) notificationEntities.AlertNotice {
	return notificationEntities.AlertNotice{
		MessageID:        n.MessageID,
		AlertID:          n.AlertID,
		AlertCode:        n.AlertCode,
		MeasuringPointID: n.MeasuringPointID,
		EventType:        n.EventType,
		StartedAt:        n.StartedAt,
		EvaluatedAt:      n.EvaluatedAt,
		Code:             n.Code,
		Level:            n.Level,
		MaxLevel:         n.MaxLevel,
		Classification:   n.Classification,
		Title:            n.Title,
		Body:             n.Body,
		Late:             n.Late,
		OccurredAt:       n.OccurredAt,
	}
}

func toAlertChange(c alertContract.AlertChanged) notificationEntities.AlertChange {
	return notificationEntities.AlertChange{
		AlertID:          c.AlertID,
		AlertCode:        c.AlertCode,
		MeasuringPointID: c.MeasuringPointID,
		EventType:        c.EventType,
		Change:           c.Change,
		State:            c.State,
		CurrentLevel:     c.CurrentLevel,
		MaxLevel:         c.MaxLevel,
		Blinking:         c.Blinking,
		ManagementStatus: c.ManagementStatus,
		Classification:   c.Classification,
		StartedAt:        c.StartedAt,
		EvaluatedAt:      c.EvaluatedAt,
		OccurredAt:       c.OccurredAt,
	}
}
