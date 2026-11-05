package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	notificationPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/fcm"
	notificationRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Push de prueba (cmd/notifications -test-send): un mensaje con el formato del
// contrato a UN celular, para que el equipo móvil compruebe la entrega sin
// esperar una alerta. Usa la misma configuración de FCM que la plataforma:
// con FCM_VALIDATE_ONLY sin definir o en true, FCM valida y no entrega.

// testPushAlertCode no existe en la base: la app no encuentra su detalle.
const testPushAlertCode = "ALR-PRUEBA-000001"

// ErrTestPushDeviceNotAllowed: el celular no está en NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST.
var ErrTestPushDeviceNotAllowed = errors.New("el celular no está en la allowlist de push")

// ErrTestPushDeviceNotRegistered: el celular no registró su token (PUT /api/devices/push-token).
var ErrTestPushDeviceNotRegistered = errors.New("el celular no tiene token registrado")

// TestPushSender es el sender de FCM con la configuración del entorno. A
// diferencia de la plataforma, sin credencial falla: no hay nada que probar.
type TestPushSender struct {
	notificationPorts.PushSender
	ValidateOnly bool
	ProjectID    string
}

// NewTestPushSender arma el sender con la credencial y FCM_VALIDATE_ONLY del entorno.
func NewTestPushSender(ctx context.Context) (*TestPushSender, error) {
	cfg := pushConfig(logger.New("NotificationsCLI", "Info"))
	if !cfg.HasCredential() {
		return nil, fmt.Errorf("%w: definir %s o %s", fcm.ErrNoCredential, FCMServiceAccountFileEnv, FCMServiceAccountBase64Env)
	}
	sender, err := fcm.NewHTTPV1Sender(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &TestPushSender{PushSender: sender, ValidateOnly: cfg.ValidateOnly, ProjectID: cfg.ProjectID}, nil
}

// TestPushToken busca el token registrado del celular en push_devices,
// respetando la allowlist del entorno.
func TestPushToken(ctx context.Context, platform *Platform, deviceID string) (string, error) {
	if err := MigrateNotifications(platform.DB); err != nil {
		return "", err
	}
	log := logger.New("NotificationsCLI", "Info")
	return testPushToken(ctx, notificationRepositories.NewPushDeviceRepository(platform.DB), pushAllowlist(log), deviceID)
}

func testPushToken(ctx context.Context, tokens notificationPorts.PushTokenRepository, allowlist []string, deviceID string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if len(allowlist) > 0 && !slices.Contains(allowlist, deviceID) {
		return "", fmt.Errorf("%w: %s", ErrTestPushDeviceNotAllowed, deviceID)
	}
	devices, err := tokens.List(ctx)
	if err != nil {
		return "", fmt.Errorf("leer push_devices: %w", err)
	}
	for _, d := range devices {
		if d.DeviceID == deviceID {
			return d.Token, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrTestPushDeviceNotRegistered, deviceID)
}

// NewTestPushMessage arma el mensaje de prueba. level: YELLOW, ORANGE o RED;
// kind: NEW o LEVEL_UP (sin distinguir mayúsculas). El messageId es negativo,
// como en los eventos de prueba, y distinto en cada envío para que la app no
// lo descarte como repetido.
func NewTestPushMessage(level, kind string, now time.Time) (notificationEntities.PushMessage, error) {
	pushLevel := notificationEntities.PushLevel(strings.ToUpper(strings.TrimSpace(level)))
	switch pushLevel {
	case notificationEntities.PushLevelYellow, notificationEntities.PushLevelOrange, notificationEntities.PushLevelRed:
	default:
		return notificationEntities.PushMessage{}, fmt.Errorf("nivel %q inválido: YELLOW, ORANGE o RED", level)
	}
	pushKind := notificationEntities.PushKind(strings.ToUpper(strings.TrimSpace(kind)))
	switch pushKind {
	case notificationEntities.PushKindNew, notificationEntities.PushKindLevelUp:
	default:
		return notificationEntities.PushMessage{}, fmt.Errorf("tipo %q inválido: NEW o LEVEL_UP", kind)
	}
	return notificationEntities.PushMessage{
		MessageID: -now.UnixMilli(),
		AlertID:   testPushAlertCode,
		Level:     pushLevel,
		Kind:      pushKind,
		SensorID:  "Sensor PK34-470",
		PK:        "34+470",
		Event:     notificationEntities.PushEventLeak,
		StartedAt: now.Truncate(time.Minute),
	}, nil
}
