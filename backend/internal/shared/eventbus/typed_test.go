package eventbus_test

import (
	"context"
	"slices"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type sampleEvent struct {
	ID int
}

func TestTyped_PublishAndSubscribe(t *testing.T) {
	bus := newBus(t)
	rec := &recorder[sampleEvent]{}
	eventbus.SubscribeTyped(bus, topic, func(_ context.Context, e sampleEvent) { rec.add(e) },
		logger.New("eventbus-test", "INFO"))

	eventbus.PublishTyped(context.Background(), bus, topic, sampleEvent{ID: 7})

	waitFor(t, "evento tipado", func() bool { return rec.len() == 1 })
	if got := rec.snapshot()[0]; got.ID != 7 {
		t.Fatalf("evento recibido = %+v", got)
	}
}

func TestTyped_MismatchIsLoggedAndDiscarded(t *testing.T) {
	out := captureLog(t)
	bus := newBus(t)
	rec := &recorder[string]{}
	eventbus.SubscribeTyped(bus, topic, func(_ context.Context, e string) { rec.add(e) }, nil,
		eventbus.WithName("typed"))

	bus.Publish(context.Background(), topic, 42)
	bus.Publish(context.Background(), topic, "ok")

	waitFor(t, "evento válido", func() bool { return rec.len() == 1 })
	if got := rec.snapshot(); !slices.Equal(got, []string{"ok"}) {
		t.Fatalf("recibido %v, se esperaba [ok]", got)
	}
	if out.count("evento con tipo inesperado descartado") != 1 {
		t.Fatal("la aserción fallida debía registrarse como error")
	}
	if st := statsFor(t, bus, "typed"); st.Panics != 0 {
		t.Fatalf("la aserción fallida no debe provocar pánicos: %+v", st)
	}
}
