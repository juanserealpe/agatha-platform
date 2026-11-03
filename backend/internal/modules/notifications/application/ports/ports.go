// Package ports son las dependencias de los casos de uso de notificaciones.
// PushSender lo implementa infrastructure/fcm (Firebase Cloud Messaging).
package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// PushSender entrega un Push a los dispositivos y devuelve el ID que asignó
// el proveedor.
type PushSender interface {
	Send(ctx context.Context, push entities.Push) (providerID string, err error)
}
