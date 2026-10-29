package eventbus_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// syncBuffer es un io.Writer seguro con concurrencia para capturar logs.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) count(substr string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Count(s.buf.String(), substr)
}

// captureLog redirige la salida del logger estándar durante el test.
func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	out := &syncBuffer{}
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(io.Discard) })
	return out
}

// newBus crea un bus que se cierra automáticamente al terminar el test.
func newBus(t *testing.T, opts ...eventbus.BusOption) *eventbus.InMemoryBus {
	t.Helper()
	bus := eventbus.NewInMemory(logger.New("eventbus-test", "INFO"), opts...)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = bus.Close(ctx)
	})
	return bus
}

// waitFor espera hasta que cond sea verdadera o falla el test.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timeout esperando: %s", what)
}

// recorder acumula eventos recibidos de forma segura.
type recorder[T any] struct {
	mu     sync.Mutex
	events []T
}

func (r *recorder[T]) add(e T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder[T]) snapshot() []T {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]T(nil), r.events...)
}

func (r *recorder[T]) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

// blockingHandler registra eventos int y bloquea en gate; avisa en started
// cada vez que empieza a procesar uno.
func blockingHandler(rec *recorder[int], started chan<- struct{}, gate <-chan struct{}) eventbus.Handler {
	return func(ctx context.Context, event any) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-gate
		rec.add(event.(int))
	}
}

func statsFor(t *testing.T, bus eventbus.Bus, name string) eventbus.SubscriptionStats {
	t.Helper()
	for _, s := range bus.Stats() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no hay stats para la suscripción %q", name)
	return eventbus.SubscriptionStats{}
}
