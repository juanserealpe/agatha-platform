// Package ports son los puertos de Notifications: los canales de entrega los
// implementa infrastructure (hub SSE, FCM) y los cablea internal/bootstrap.
package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// NoticeChannel entrega un aviso por un canal (web: el hub SSE; móvil: FCM).
type NoticeChannel interface {
	DeliverNotice(ctx context.Context, n entities.AlertNotice) error
}

// ChangeChannel entrega la foto de una alerta. Solo lo implementa el canal web.
type ChangeChannel interface {
	DeliverChange(ctx context.Context, c entities.AlertChange) error
}

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
