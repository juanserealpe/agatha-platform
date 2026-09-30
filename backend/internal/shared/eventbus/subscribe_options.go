package eventbus

import "time"

// SubscribeOption ajusta la configuración de una suscripción.
// Los valores inválidos se ignoran y se conserva el default del bus.
type SubscribeOption func(*subscribeConfig)

// WithName asigna un nombre legible a la suscripción (aparece en logs y Stats).
func WithName(name string) SubscribeOption {
	return func(c *subscribeConfig) {
		if name != "" {
			c.name = name
		}
	}
}

// WithBuffer fija la capacidad de la cola de la suscripción (debe ser > 0).
func WithBuffer(size int) SubscribeOption {
	return func(c *subscribeConfig) {
		if size > 0 {
			c.buffer = size
		}
	}
}

// WithOverflow fija la política aplicada cuando la cola está llena.
func WithOverflow(policy OverflowPolicy) SubscribeOption {
	return func(c *subscribeConfig) {
		if policy.valid() {
			c.overflow = policy
		}
	}
}

// WithHandlerTimeout fija el tiempo máximo del contexto entregado al handler (debe ser > 0).
func WithHandlerTimeout(timeout time.Duration) SubscribeOption {
	return func(c *subscribeConfig) {
		if timeout > 0 {
			c.handlerTimeout = timeout
		}
	}
}
