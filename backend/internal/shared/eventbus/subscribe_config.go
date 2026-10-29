package eventbus

import "time"

// subscribeConfig agrupa la configuración resuelta de una suscripción.
type subscribeConfig struct {
	name           string
	buffer         int
	overflow       OverflowPolicy
	handlerTimeout time.Duration
}
