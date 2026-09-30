package eventbus

// OverflowPolicy define qué evento se descarta cuando la cola de una
// suscripción está llena.
type OverflowPolicy int

const (
	// DropOldest descarta el evento más antiguo de la cola y encola el nuevo (default).
	DropOldest OverflowPolicy = iota
	// DropNewest descarta el evento entrante y conserva la cola intacta.
	DropNewest
)

// String devuelve el nombre legible de la política.
func (p OverflowPolicy) String() string {
	switch p {
	case DropOldest:
		return "drop_oldest"
	case DropNewest:
		return "drop_newest"
	default:
		return "unknown"
	}
}

// valid indica si la política es una de las soportadas.
func (p OverflowPolicy) valid() bool {
	return p == DropOldest || p == DropNewest
}
