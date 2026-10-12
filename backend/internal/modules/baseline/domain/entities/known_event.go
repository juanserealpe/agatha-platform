package entities

import "time"

// KnownEvent es un intervalo [From, To) que NO debe usarse para calcular la
// línea base porque corresponde a un evento conocido (fuga, intervención,
// reparación…). A diferencia de ExcludedPeriod no lo decide el barrido de I:
// se excluye siempre, aunque el barrido no lo detecte.
type KnownEvent struct {
	From  time.Time
	To    time.Time
	Label string
}
