package eventbus

// SubscriptionStats es una foto de los contadores de una suscripción.
//
// Invariante tras drenar: Published = Delivered + Panics + Dropped.
type SubscriptionStats struct {
	// Name es el nombre de la suscripción (WithName o "<topic>#<id>").
	Name string
	// Topic es el tópico suscrito.
	Topic string
	// Published cuenta los eventos ofrecidos a la cola (incluye los descartados).
	Published uint64
	// Delivered cuenta las ejecuciones del handler que terminaron sin pánico.
	Delivered uint64
	// Dropped cuenta los eventos descartados por overflow, cancelación o cierre.
	Dropped uint64
	// Panics cuenta los pánicos recuperados del handler.
	Panics uint64
	// QueueLen es la cantidad de eventos pendientes en la cola.
	QueueLen int
}
