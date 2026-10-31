// Package ports son los puertos de Notifications: los canales de entrega los
// implementa infrastructure (hub SSE, FCM) y los cablea internal/bootstrap.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// Clock da la hora actual (fija en las pruebas).
type Clock interface {
	Now() time.Time
}

// ClockFunc adapta una función a Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

// NoticeChannel entrega un aviso por un canal (web: el hub SSE; móvil: FCM).
type NoticeChannel interface {
	DeliverNotice(ctx context.Context, n entities.AlertNotice) error
}

// ChangeChannel entrega la foto de una alerta. Solo lo implementa el canal web.
type ChangeChannel interface {
	DeliverChange(ctx context.Context, c entities.AlertChange) error
}

// PushSender envía un mensaje a un celular (en producción, FCM HTTP v1).
// Errores: ErrPushTokenUnregistered (borrar el token), *RetryablePushError
// (reintentar) o cualquier otro (definitivo para ese mensaje).
type PushSender interface {
	Send(ctx context.Context, token string, m entities.PushMessage) error
}

// ErrPushTokenUnregistered: FCM ya no reconoce el token (UNREGISTERED o 404):
// la app se desinstaló o el token caducó. Hay que borrarlo.
var ErrPushTokenUnregistered = errors.New("FCM ya no reconoce el token de push")

// RetryablePushError: el envío falló por algo pasajero (429, 5xx, red).
// RetryAfter es lo que pidió FCM con la cabecera Retry-After (0 si no pidió).
type RetryablePushError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *RetryablePushError) Error() string { return e.Err.Error() }
func (e *RetryablePushError) Unwrap() error { return e.Err }

// PushTokenRepository guarda el token FCM vigente de cada celular
// (push_devices). Un celular tiene un solo token y un token es de un solo
// celular.
type PushTokenRepository interface {
	// Upsert guarda el token del celular, reemplazando el anterior. Si el token
	// era de otro deviceId, pasa a este. Es idempotente (la app reintenta).
	Upsert(ctx context.Context, d entities.PushDevice) error
	// List devuelve todos los celulares registrados, ordenados por deviceId.
	List(ctx context.Context) ([]entities.PushDevice, error)
	// DeleteToken borra el token (FCM respondió UNREGISTERED o 404). Si el
	// celular ya registró otro token, no lo toca.
	DeleteToken(ctx context.Context, token string) error
}
