// Package eventbus provee un bus de eventos técnico, en memoria y en proceso,
// para comunicar módulos del monolito sin acoplarlos entre sí.
//
// Semántica: best-effort. Los eventos son pistas; la base de datos es la
// fuente de verdad. Ver README.md para garantías y límites.
package eventbus

import "context"

// Bus publica eventos por tópico y los entrega a los suscriptores registrados.
type Bus interface {
	// Publish entrega el evento a la cola de cada suscripción del tópico.
	// Nunca bloquea ni falla al productor; sin suscriptores es un no-op.
	Publish(ctx context.Context, topic string, event any)

	// Subscribe registra un handler para el tópico y devuelve la función
	// para cancelar la suscripción (idempotente y segura con concurrencia).
	Subscribe(topic string, h Handler, opts ...SubscribeOption) (unsubscribe func())

	// Stats devuelve una foto de los contadores de cada suscripción activa.
	Stats() []SubscriptionStats

	// Close deja de aceptar eventos y drena las colas hasta el deadline de ctx.
	// Si el deadline expira, descarta lo pendiente y devuelve ctx.Err().
	Close(ctx context.Context) error
}
