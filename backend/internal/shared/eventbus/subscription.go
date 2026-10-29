package eventbus

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// warnInterval limita los warn repetitivos (overflow, publicación tras cierre).
const warnInterval = 5 * time.Second

// subscription es la cola acotada (anillo) + worker FIFO de un suscriptor.
//
// Ciclo de vida: activa → drenando (Close) → detenida; o activa → detenida
// (unsubscribe o deadline de Close). Al detenerse descarta lo pendiente.
type subscription struct {
	topic   string
	cfg     subscribeConfig
	handler Handler
	log     *logger.Logger

	mu       sync.Mutex
	buf      []envelope
	head     int
	size     int
	draining bool
	lastWarn time.Time

	// notify despierta al worker (capacidad 1, envío no bloqueante).
	notify chan struct{}
	// stopCtx se cancela al detener la suscripción; también cancela el handler en curso.
	stopCtx context.Context
	stop    context.CancelFunc

	published atomic.Uint64
	delivered atomic.Uint64
	dropped   atomic.Uint64
	panics    atomic.Uint64
}

// newSubscription crea la suscripción; el worker se arranca con run.
func newSubscription(topic string, h Handler, cfg subscribeConfig, log *logger.Logger) *subscription {
	stopCtx, stop := context.WithCancel(context.Background())

	return &subscription{
		topic:   topic,
		cfg:     cfg,
		handler: h,
		log:     log,
		buf:     make([]envelope, cfg.buffer),
		notify:  make(chan struct{}, 1),
		stopCtx: stopCtx,
		stop:    stop,
	}
}

// enqueue ofrece un evento a la cola sin bloquear. Aplica la política de
// overflow si está llena. Devuelve false si la suscripción ya no acepta eventos.
func (s *subscription) enqueue(env envelope) bool {
	s.mu.Lock()
	if s.draining || s.stopCtx.Err() != nil {
		s.mu.Unlock()
		return false
	}

	s.published.Add(1)
	warn := false
	if s.size == len(s.buf) {
		s.dropped.Add(1)
		if s.cfg.overflow == DropOldest {
			s.popLocked()
			s.pushLocked(env)
		}
		warn = s.shouldWarnLocked(time.Now())
	} else {
		s.pushLocked(env)
	}
	s.mu.Unlock()

	if warn {
		s.log.Warn(
			"cola de suscripción llena; eventos descartados",
			"topic", s.topic,
			"subscription", s.cfg.name,
			"policy", s.cfg.overflow.String(),
			"dropped_total", s.dropped.Load(),
		)
	}
	s.wake()

	return true
}

// run es el loop del worker: entrega en orden FIFO hasta que la cola se
// cierre (drenada) o la suscripción se detenga.
func (s *subscription) run() {
	for {
		env, ok := s.next()
		if !ok {
			return
		}
		s.deliver(env)
	}
}

// next bloquea hasta obtener el siguiente evento. Devuelve false cuando el
// worker debe terminar.
func (s *subscription) next() (envelope, bool) {
	for {
		s.mu.Lock()
		if s.stopCtx.Err() != nil {
			s.discardPendingLocked()
			s.mu.Unlock()
			return envelope{}, false
		}
		if s.size > 0 {
			env := s.popLocked()
			s.mu.Unlock()
			return env, true
		}
		if s.draining {
			s.mu.Unlock()
			return envelope{}, false
		}
		s.mu.Unlock()

		select {
		case <-s.notify:
		case <-s.stopCtx.Done():
		}
	}
}

// deliver ejecuta el handler con un contexto desconectado de la cancelación
// del publicador, con timeout, y recupera pánicos.
func (s *subscription) deliver(env envelope) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(env.ctx), s.cfg.handlerTimeout)
	stopAfter := context.AfterFunc(s.stopCtx, cancel)

	defer func() {
		stopAfter()
		cancel()

		if r := recover(); r != nil {
			s.panics.Add(1)
			s.log.Error(
				"pánico recuperado en handler de evento",
				"topic", s.topic,
				"subscription", s.cfg.name,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			return
		}
		s.delivered.Add(1)
	}()

	s.handler(ctx, env.event)
}

// startDrain deja de aceptar eventos; el worker termina al vaciar la cola.
func (s *subscription) startDrain() {
	s.mu.Lock()
	s.draining = true
	s.mu.Unlock()
	s.wake()
}

// terminate detiene la suscripción: descarta lo pendiente y cancela el
// contexto del handler en curso. Idempotente.
func (s *subscription) terminate() {
	s.stop()
}

// stats devuelve la foto de contadores de la suscripción.
func (s *subscription) stats() SubscriptionStats {
	s.mu.Lock()
	queueLen := s.size
	s.mu.Unlock()

	return SubscriptionStats{
		Name:      s.cfg.name,
		Topic:     s.topic,
		Published: s.published.Load(),
		Delivered: s.delivered.Load(),
		Dropped:   s.dropped.Load(),
		Panics:    s.panics.Load(),
		QueueLen:  queueLen,
	}
}

// wake despierta al worker sin bloquear.
func (s *subscription) wake() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// pushLocked agrega al final del anillo. Requiere s.mu y espacio disponible.
func (s *subscription) pushLocked(env envelope) {
	s.buf[(s.head+s.size)%len(s.buf)] = env
	s.size++
}

// popLocked extrae el primero del anillo. Requiere s.mu y size > 0.
func (s *subscription) popLocked() envelope {
	env := s.buf[s.head]
	s.buf[s.head] = envelope{}
	s.head = (s.head + 1) % len(s.buf)
	s.size--
	return env
}

// discardPendingLocked vacía la cola contabilizando los descartes. Requiere s.mu.
func (s *subscription) discardPendingLocked() {
	if s.size == 0 {
		return
	}
	s.dropped.Add(uint64(s.size))
	for s.size > 0 {
		s.popLocked()
	}
}

// shouldWarnLocked aplica el rate-limit de warn por suscripción. Requiere s.mu.
func (s *subscription) shouldWarnLocked(now time.Time) bool {
	if now.Sub(s.lastWarn) < warnInterval {
		return false
	}
	s.lastWarn = now
	return true
}
