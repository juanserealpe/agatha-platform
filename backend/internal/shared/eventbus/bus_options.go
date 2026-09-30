package eventbus

import "time"

const (
	// DefaultBuffer es la capacidad de cola por suscripción cuando no se configura otra.
	DefaultBuffer = 256
	// DefaultHandlerTimeout es el timeout por handler cuando no se configura otro.
	DefaultHandlerTimeout = 30 * time.Second
)

// BusOption ajusta los defaults de un InMemoryBus.
// Los valores inválidos se ignoran.
type BusOption func(*InMemoryBus)

// WithDefaultBuffer fija la capacidad de cola por defecto de las suscripciones (debe ser > 0).
func WithDefaultBuffer(size int) BusOption {
	return func(b *InMemoryBus) {
		if size > 0 {
			b.defaultBuffer = size
		}
	}
}

// WithDefaultHandlerTimeout fija el timeout por defecto de los handlers (debe ser > 0).
func WithDefaultHandlerTimeout(timeout time.Duration) BusOption {
	return func(b *InMemoryBus) {
		if timeout > 0 {
			b.defaultHandlerTimeout = timeout
		}
	}
}
