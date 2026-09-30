package eventbus

import (
	"context"
	"reflect"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// PublishTyped publica un evento de tipo T; azúcar tipado sobre Bus.Publish.
func PublishTyped[T any](ctx context.Context, bus Bus, topic string, event T) {
	bus.Publish(ctx, topic, event)
}

// SubscribeTyped suscribe un handler que recibe eventos de tipo T.
//
// Si llega un evento que no es T se registra como error en log y se descarta
// sin invocar al handler (nunca entra en pánico).
func SubscribeTyped[T any](
	bus Bus,
	topic string,
	handler func(ctx context.Context, event T),
	log *logger.Logger,
	opts ...SubscribeOption,
) (unsubscribe func()) {
	if log == nil {
		log = logger.New(loggerModule, "INFO")
	}

	return bus.Subscribe(topic, func(ctx context.Context, event any) {
		typed, ok := event.(T)
		if !ok {
			log.Error(
				"evento con tipo inesperado descartado",
				"topic", topic,
				"expected", reflect.TypeFor[T]().String(),
				"received", reflect.TypeOf(event),
			)
			return
		}
		handler(ctx, typed)
	}, opts...)
}
