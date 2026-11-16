package bootstrap

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	notificationPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	notificationServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/policies"
	notificationEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/fcm"
	notificationHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/http"
	notificationPersistence "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence"
	notificationRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/realtime"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Variables de entorno de Notifications. La credencial de FCM nunca va al log.
const (
	// NotificationsSSEMaxClientsEnv es el tope de streams SSE abiertos a la vez.
	NotificationsSSEMaxClientsEnv = "NOTIFICATIONS_SSE_MAX_CLIENTS"
	// NotificationsPushAllowlistEnv limita el push a esos deviceId (separados por coma).
	NotificationsPushAllowlistEnv = "NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST"
	FCMProjectIDEnv               = "FCM_PROJECT_ID"
	FCMServiceAccountFileEnv      = "FCM_SERVICE_ACCOUNT_FILE"
	FCMServiceAccountBase64Env    = "FCM_SERVICE_ACCOUNT_JSON_BASE64"
	// FCMValidateOnlyEnv: FCM valida los mensajes sin entregarlos (local).
	FCMValidateOnlyEnv = "FCM_VALIDATE_ONLY"
)

// defaultFCMProjectID es el proyecto Firebase de la app Android.
const defaultFCMProjectID = "agatha-5fe71"

// pushHandlerTimeout es el plazo de la suscripción del push: cubre los
// reintentos (hasta 4 intentos de 10 s y esperas de hasta 30 s) de los
// celulares del piloto. Es una cola propia: no frena a la web.
const pushHandlerTimeout = 3 * time.Minute

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
	// pushTokens expone el registro de tokens FCM de los celulares.
	pushTokens *notificationHTTP.PushTokenHandler
	// testEvents es el endpoint de eventos de prueba (nil si no está habilitado).
	testEvents *notificationHTTP.TestEventHandler
}

// MigrateNotifications crea push_devices (idempotente y solo aditivo).
func MigrateNotifications(db *gorm.DB) error {
	if err := notificationPersistence.Migrate(db); err != nil {
		logger.New("DataBaseInit", "Info").Error("falló la migración de Notifications", "error", err)
		return err
	}
	return nil
}

// NewNotificationsModule migra y cablea Notifications. Consume los eventos de
// Alert Management, pero no lo exige: sin él no recibe nada (ver ParseModules).
func NewNotificationsModule(platform *Platform) (*NotificationsModule, error) {
	log := logger.New("Notifications", "Info")
	if err := MigrateNotifications(platform.DB); err != nil {
		return nil, err
	}
	hub := realtime.NewHub(realtime.HubConfig{
		MaxClients: envInt(log, NotificationsSSEMaxClientsEnv, realtime.DefaultMaxClients),
	}, log)
	tokens := notificationRepositories.NewPushDeviceRepository(platform.DB)
	push := notificationServices.NewMobilePushService(tokens, notificationRepositories.NewDeviceDirectory(platform.DB),
		newPushSender(log), notificationServices.MobilePushConfig{Allowlist: pushAllowlist(log)}, log)
	m := wireNotifications(platform.EventBus, notificationChannels{webNotices: hub, webChanges: hub, push: push}, log)
	m.hub = hub
	m.pushTokens = notificationHTTP.NewPushTokenHandler(
		notificationServices.NewPushTokenService(tokens, notificationPorts.ClockFunc(time.Now), log),
		log,
	)
	if testEndpointEnabled(log) {
		m.testEvents = notificationHTTP.NewTestEventHandler(
			newBusTestEventEmitter(platform.EventBus, notificationPorts.ClockFunc(time.Now)), log)
	}
	return m, nil
}

// pushConfig lee la configuración de FCM. FCM_VALIDATE_ONLY es true si no se
// define: para entregar push reales hay que pedirlo explícitamente con false
// (lo hace el docker-compose del VPS). Así un backend local con la credencial
// no llega por error a los celulares del piloto.
func pushConfig(log *logger.Logger) fcm.Config {
	cfg := fcm.Config{
		ProjectID:                strings.TrimSpace(os.Getenv(FCMProjectIDEnv)),
		ServiceAccountFile:       os.Getenv(FCMServiceAccountFileEnv),
		ServiceAccountJSONBase64: os.Getenv(FCMServiceAccountBase64Env),
		ValidateOnly:             envBool(log, FCMValidateOnlyEnv, true),
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = defaultFCMProjectID
	}
	return cfg
}

// newPushSender arma el sender de FCM. Sin credencial, o si la credencial no
// sirve, usa NoopSender: el push queda deshabilitado pero la plataforma (y el
// canal web) arrancan igual.
func newPushSender(log *logger.Logger) notificationPorts.PushSender {
	cfg := pushConfig(log)
	if !cfg.HasCredential() {
		log.Warn("push FCM deshabilitado: falta la credencial", "file_env", FCMServiceAccountFileEnv, "base64_env", FCMServiceAccountBase64Env)
		return fcm.NewNoopSender(log)
	}
	sender, err := fcm.NewHTTPV1Sender(context.Background(), cfg)
	if err != nil {
		log.Error("push FCM deshabilitado: la credencial no sirve", "error", err)
		return fcm.NewNoopSender(log)
	}
	switch {
	case cfg.ValidateOnly:
		log.Info("push FCM en modo validación: FCM valida los mensajes y no los entrega", "project_id", cfg.ProjectID,
			"enable_env", FCMValidateOnlyEnv+"=false")
	case strings.Trim(os.Getenv(NotificationsPushAllowlistEnv), " ,") == "" || isProductionEnvironment():
		log.Warn("push FCM real habilitado para TODOS los celulares registrados", "project_id", cfg.ProjectID)
	default:
		log.Info("push FCM real habilitado solo para la allowlist", "project_id", cfg.ProjectID)
	}
	return sender
}

// pushAllowlist lee los deviceId permitidos (vacía = todos los celulares). Es
// solo para probar desde local: en producción (AGATHA_ENV=production) se
// ignora, porque una allowlist olvidada en el .env deja sin push a todo
// celular que no esté en ella (así se perdió el push de AGATHA QA, 13/11/2026).
func pushAllowlist(log *logger.Logger) []string {
	var ids []string
	for _, part := range strings.Split(os.Getenv(NotificationsPushAllowlistEnv), ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if isProductionEnvironment() {
		log.Warn("allowlist de push ignorada en producción: el push va a todos los celulares registrados",
			"env", NotificationsPushAllowlistEnv, "ignored_devices", len(ids))
		return nil
	}
	log.Warn("push limitado a la allowlist", "devices", strings.Join(ids, ","))
	return ids
}

func envBool(log *logger.Logger, key string, def bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		log.Warn("variable de entorno inválida; se usa el valor por defecto", "key", key, "value", raw, "default", def)
		return def
	}
	return v
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
			notificationServices.NewNoticeDispatcher("push", policies.MobilePushPolicy{}, m.channels.push, m.log), m.log,
			eventbus.WithHandlerTimeout(pushHandlerTimeout)))
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

// RegisterRoutes expone GET /api/notifications/stream, PUT /api/devices/push-token
// y, si está habilitado, POST /api/notifications/test-event.
func (m *NotificationsModule) RegisterRoutes(mux *http.ServeMux) {
	if m.hub != nil {
		realtime.NewStreamHandler(m.hub, m.log).RegisterRoutes(mux)
	}
	if m.pushTokens != nil {
		m.pushTokens.RegisterRoutes(mux)
	}
	if m.testEvents != nil {
		m.testEvents.RegisterRoutes(mux)
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
func subscribeAlertNotification(bus eventbus.Bus, name string, dispatcher notificationEvents.NoticeDispatcher, log *logger.Logger,
	opts ...eventbus.SubscribeOption) (unsubscribe func()) {
	subscriber := notificationEvents.NewAlertNotificationSubscriber(dispatcher)
	return eventbus.SubscribeTyped(bus, alertContract.TopicAlertNotification,
		func(ctx context.Context, n alertContract.AlertNotification) {
			subscriber.Handle(ctx, toAlertNotice(n))
		},
		log,
		append([]eventbus.SubscribeOption{eventbus.WithName(name)}, opts...)...,
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
