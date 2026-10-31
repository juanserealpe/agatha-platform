package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Valores por defecto del canal móvil.
const (
	DefaultPushMaxAttempts   = 4                // 1 envío + 3 reintentos
	DefaultPushBaseBackoff   = time.Second      // 1 s, 2 s, 4 s
	DefaultPushMaxRetryAfter = 30 * time.Second // tope a lo que pida FCM
	DefaultPushConcurrency   = 4                // celulares en paralelo
)

// MobilePushConfig ajusta los reintentos y el alcance del canal móvil.
type MobilePushConfig struct {
	MaxAttempts   int
	BaseBackoff   time.Duration
	MaxRetryAfter time.Duration
	Concurrency   int
	// Allowlist limita el envío a esos deviceId (vacía = todos). Sirve para
	// probar desde local sin llegar a los celulares del piloto.
	Allowlist []string
	// Sleep espera entre reintentos; nil = espera real (las pruebas no esperan).
	Sleep func(ctx context.Context, d time.Duration) error
}

func (c MobilePushConfig) withDefaults() MobilePushConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultPushMaxAttempts
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = DefaultPushBaseBackoff
	}
	if c.MaxRetryAfter <= 0 {
		c.MaxRetryAfter = DefaultPushMaxRetryAfter
	}
	if c.Concurrency <= 0 {
		c.Concurrency = DefaultPushConcurrency
	}
	if c.Sleep == nil {
		c.Sleep = sleep
	}
	return c
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// MobilePushService es el canal móvil (implementa ports.NoticeChannel): arma
// el mensaje, lo envía a cada celular registrado con reintentos y borra los
// tokens que FCM ya no reconoce. La política (qué avisos van) la aplica antes
// el NoticeDispatcher.
type MobilePushService struct {
	tokens  ports.PushTokenRepository
	devices ports.DeviceDirectory
	sender  ports.PushSender
	cfg     MobilePushConfig
	allow   map[string]bool
	log     *logger.Logger
}

func NewMobilePushService(tokens ports.PushTokenRepository, devices ports.DeviceDirectory, sender ports.PushSender,
	cfg MobilePushConfig, log *logger.Logger) *MobilePushService {
	cfg = cfg.withDefaults()
	var allow map[string]bool
	if len(cfg.Allowlist) > 0 {
		allow = make(map[string]bool, len(cfg.Allowlist))
		for _, id := range cfg.Allowlist {
			allow[id] = true
		}
	}
	return &MobilePushService{tokens: tokens, devices: devices, sender: sender, cfg: cfg, allow: allow, log: log}
}

// pushOutcome es el resultado del envío a un celular.
type pushOutcome int

const (
	pushDelivered pushOutcome = iota
	pushTokenRemoved
	pushFailed
)

// DeliverNotice envía el aviso a todos los celulares (o a los de la allowlist).
// Devuelve error si algún envío falló del todo; los tokens borrados no cuentan
// como falla. El token nunca va al log: se identifica por deviceId.
func (s *MobilePushService) DeliverNotice(ctx context.Context, n entities.AlertNotice) error {
	name, err := s.devices.DeviceName(ctx, n.MeasuringPointID)
	if err != nil {
		// sensorId y pk son opcionales: se envía sin ellos.
		s.log.Warn("[MobilePushService] No se pudo leer el dispositivo; el push va sin sensorId ni pk",
			"measuring_point_id", n.MeasuringPointID, "error", err)
	}
	msg, err := entities.NewPushMessage(n, name)
	if err != nil {
		return fmt.Errorf("armar el push de %s: %w", n.AlertCode, err)
	}

	devices, err := s.tokens.List(ctx)
	if err != nil {
		return fmt.Errorf("listar los celulares registrados: %w", err)
	}
	devices = s.allowed(devices)
	if len(devices) == 0 {
		s.log.Info("[MobilePushService] No hay celulares registrados para el push", "alert", n.AlertCode, "message_id", n.MessageID)
		return nil
	}

	outcomes := make([]pushOutcome, len(devices))
	sem := make(chan struct{}, s.cfg.Concurrency)
	var wg sync.WaitGroup
	for i, d := range devices {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			outcomes[i] = s.deliverTo(ctx, d, msg, n)
		}()
	}
	wg.Wait()

	counts := map[pushOutcome]int{}
	for _, o := range outcomes {
		counts[o]++
	}
	s.log.Info("[MobilePushService] Push enviado", "alert", n.AlertCode, "message_id", n.MessageID, "kind", msg.Kind,
		"delivered", counts[pushDelivered], "tokens_removed", counts[pushTokenRemoved], "failed", counts[pushFailed])
	if counts[pushFailed] > 0 {
		return fmt.Errorf("el push de %s falló en %d de %d celulares", n.AlertCode, counts[pushFailed], len(devices))
	}
	return nil
}

func (s *MobilePushService) allowed(devices []entities.PushDevice) []entities.PushDevice {
	if s.allow == nil {
		return devices
	}
	out := devices[:0:0]
	for _, d := range devices {
		if s.allow[d.DeviceID] {
			out = append(out, d)
		}
	}
	return out
}

// deliverTo envía a un celular con reintentos y borra su token si FCM ya no lo reconoce.
func (s *MobilePushService) deliverTo(ctx context.Context, d entities.PushDevice, msg entities.PushMessage, n entities.AlertNotice) pushOutcome {
	err := s.sendWithRetry(ctx, d.Token, msg)
	switch {
	case err == nil:
		return pushDelivered
	case errors.Is(err, ports.ErrPushTokenUnregistered):
		if delErr := s.tokens.DeleteToken(ctx, d.Token); delErr != nil {
			s.log.Error("[MobilePushService] No se pudo borrar un token que FCM ya no reconoce",
				"device_id", d.DeviceID, "error", delErr)
			return pushFailed
		}
		s.log.Info("[MobilePushService] Token borrado: FCM ya no lo reconoce", "device_id", d.DeviceID)
		return pushTokenRemoved
	default:
		s.log.Error("[MobilePushService] No se pudo enviar el push",
			"device_id", d.DeviceID, "alert", n.AlertCode, "message_id", n.MessageID, "error", err)
		return pushFailed
	}
}

// sendWithRetry reintenta solo los *ports.RetryablePushError, con backoff
// exponencial (1 s, 2 s, 4 s…) o lo que pida Retry-After si es mayor (con tope).
func (s *MobilePushService) sendWithRetry(ctx context.Context, token string, msg entities.PushMessage) error {
	for attempt := 1; ; attempt++ {
		err := s.sender.Send(ctx, token, msg)
		var retryable *ports.RetryablePushError
		if err == nil || !errors.As(err, &retryable) || attempt >= s.cfg.MaxAttempts {
			return err
		}
		wait := s.cfg.BaseBackoff << (attempt - 1)
		if retryable.RetryAfter > wait {
			wait = min(retryable.RetryAfter, s.cfg.MaxRetryAfter)
		}
		if err := s.cfg.Sleep(ctx, wait); err != nil {
			return err
		}
	}
}
