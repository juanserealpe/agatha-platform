package eventbus

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// loggerModule es el nombre de módulo usado por el logger por defecto del bus.
const loggerModule = "eventbus"

// InMemoryBus implementa Bus en memoria: una cola acotada y una goroutine
// por suscripción, con orden FIFO por suscriptor.
type InMemoryBus struct {
	log                   *logger.Logger
	defaultBuffer         int
	defaultHandlerTimeout time.Duration

	mu     sync.RWMutex
	subs   map[string]map[uint64]*subscription
	nextID uint64
	closed bool

	workers       sync.WaitGroup
	activeWorkers atomic.Int64
	// lastClosedWarn guarda (UnixNano) el último warn por publicar tras Close.
	lastClosedWarn atomic.Int64
}

var _ Bus = (*InMemoryBus)(nil)

// NewInMemory crea un bus en memoria. Si log es nil usa un logger propio.
func NewInMemory(log *logger.Logger, opts ...BusOption) *InMemoryBus {
	if log == nil {
		log = logger.New(loggerModule, "INFO")
	}

	b := &InMemoryBus{
		log:                   log,
		defaultBuffer:         DefaultBuffer,
		defaultHandlerTimeout: DefaultHandlerTimeout,
		subs:                  make(map[string]map[uint64]*subscription),
	}
	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Publish encola el evento en cada suscripción del tópico sin bloquear.
// Tras Close descarta el evento con un warn (rate-limited).
func (b *InMemoryBus) Publish(ctx context.Context, topic string, event any) {
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		b.warnPublishAfterClose(topic)
		return
	}

	env := envelope{ctx: ctx, event: event}
	for _, s := range b.subs[topic] {
		s.enqueue(env)
	}
}

// Subscribe registra el handler y arranca su worker. Tras Close, o con un
// handler nil, no registra nada y devuelve un unsubscribe no-op.
func (b *InMemoryBus) Subscribe(topic string, h Handler, opts ...SubscribeOption) (unsubscribe func()) {
	if h == nil {
		b.log.Error("suscripción ignorada: handler nil", "topic", topic)
		return func() {}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		b.log.Warn("suscripción ignorada: el bus está cerrado", "topic", topic)
		return func() {}
	}

	b.nextID++
	id := b.nextID

	cfg := subscribeConfig{
		name:           topic + "#" + strconv.FormatUint(id, 10),
		buffer:         b.defaultBuffer,
		overflow:       DropOldest,
		handlerTimeout: b.defaultHandlerTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	sub := newSubscription(topic, h, cfg, b.log)
	if b.subs[topic] == nil {
		b.subs[topic] = make(map[uint64]*subscription)
	}
	b.subs[topic][id] = sub

	b.activeWorkers.Add(1)
	b.workers.Go(func() {
		defer b.activeWorkers.Add(-1)
		sub.run()
	})

	return func() { b.unsubscribe(topic, id, sub) }
}

// Stats devuelve los contadores de las suscripciones activas, ordenados por
// tópico y nombre.
func (b *InMemoryBus) Stats() []SubscriptionStats {
	subs := b.snapshot()

	stats := make([]SubscriptionStats, 0, len(subs))
	for _, s := range subs {
		stats = append(stats, s.stats())
	}
	slices.SortFunc(stats, func(a, c SubscriptionStats) int {
		return cmp.Or(cmp.Compare(a.Topic, c.Topic), cmp.Compare(a.Name, c.Name))
	})

	return stats
}

// Close deja de aceptar eventos y suscripciones y drena las colas. Si ctx
// expira antes, detiene los workers (descartando lo pendiente y cancelando el
// ctx de los handlers en curso) y devuelve ctx.Err(). Es idempotente.
func (b *InMemoryBus) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	subs := b.snapshot()
	for _, s := range subs {
		s.startDrain()
	}

	done := make(chan struct{})
	go func() {
		b.workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		for _, s := range subs {
			s.terminate()
		}
		return nil
	case <-ctx.Done():
		for _, s := range subs {
			s.terminate()
		}
		b.log.Warn("cierre del bus sin drenar por completo; eventos pendientes descartados", "error", ctx.Err())
		return ctx.Err()
	}
}

// unsubscribe retira la suscripción y detiene su worker. Idempotente.
func (b *InMemoryBus) unsubscribe(topic string, id uint64, sub *subscription) {
	b.mu.Lock()
	if topicSubs, ok := b.subs[topic]; ok {
		delete(topicSubs, id)
		if len(topicSubs) == 0 {
			delete(b.subs, topic)
		}
	}
	b.mu.Unlock()

	sub.terminate()
}

// snapshot copia las suscripciones activas bajo lock de lectura.
func (b *InMemoryBus) snapshot() []*subscription {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var subs []*subscription
	for _, topicSubs := range b.subs {
		for _, s := range topicSubs {
			subs = append(subs, s)
		}
	}

	return subs
}

// warnPublishAfterClose registra el descarte como máximo una vez por warnInterval.
func (b *InMemoryBus) warnPublishAfterClose(topic string) {
	now := time.Now().UnixNano()
	last := b.lastClosedWarn.Load()
	if now-last < int64(warnInterval) || !b.lastClosedWarn.CompareAndSwap(last, now) {
		return
	}
	b.log.Warn("evento descartado: el bus está cerrado", "topic", topic)
}
