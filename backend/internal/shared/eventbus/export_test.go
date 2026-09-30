package eventbus

// ActiveWorkers expone a los tests la cantidad de workers vivos del bus.
func ActiveWorkers(b *InMemoryBus) int64 {
	return b.activeWorkers.Load()
}
