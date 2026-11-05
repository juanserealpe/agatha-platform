package entities

import "time"

// KnownEvent es el intervalo [From, To) de una alerta del punto. Se excluye
// siempre del cálculo de la línea base (ampliado con Config.KnownEventMargin),
// aunque el barrido de I no lo detecte. A diferencia de ExcludedPeriod no lo
// decide el barrido.
type KnownEvent struct {
	From time.Time
	To   time.Time
}
