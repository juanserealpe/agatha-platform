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
