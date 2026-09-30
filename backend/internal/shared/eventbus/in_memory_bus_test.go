package eventbus_test

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
)

const topic = "test.topic"

type ctxKey struct{}

func TestPublish_DeliversToMultipleSubscribers(t *testing.T) {
	bus := newBus(t)
	recs := []*recorder[int]{{}, {}, {}}
	for _, rec := range recs {
		bus.Subscribe(topic, func(_ context.Context, e any) { rec.add(e.(int)) })
	}
	other := &recorder[int]{}
	bus.Subscribe("other.topic", func(_ context.Context, e any) { other.add(e.(int)) })

	for i := range 5 {
		bus.Publish(context.Background(), topic, i)
	}

	for i, rec := range recs {
		waitFor(t, "entrega a todos los suscriptores", func() bool { return rec.len() == 5 })
		if got := rec.snapshot(); !slices.Equal(got, []int{0, 1, 2, 3, 4}) {
			t.Errorf("suscriptor %d recibió %v", i, got)
		}
	}
	if other.len() != 0 {
		t.Errorf("otro tópico no debía recibir eventos, recibió %v", other.snapshot())
	}
}

func TestPublish_WithoutSubscribersIsNoop(t *testing.T) {
	bus := newBus(t)
	bus.Publish(context.Background(), topic, 1)
	if len(bus.Stats()) != 0 {
		t.Fatalf("no debía haber suscripciones: %v", bus.Stats())
	}
}

func TestPublish_FIFOPerSubscriber(t *testing.T) {
	const n = 2000
	bus := newBus(t)
	rec := &recorder[int]{}
	bus.Subscribe(topic, func(_ context.Context, e any) { rec.add(e.(int)) }, eventbus.WithBuffer(n))

	want := make([]int, n)
	for i := range n {
		want[i] = i
		bus.Publish(context.Background(), topic, i)
	}

	waitFor(t, "todos los eventos", func() bool { return rec.len() == n })
	if !slices.Equal(rec.snapshot(), want) {
		t.Fatal("el orden de entrega no es FIFO")
	}
}

func TestPublish_DoesNotBlockWithSlowSubscriber(t *testing.T) {
	bus := newBus(t)
	gate := make(chan struct{})
	defer close(gate)
	bus.Subscribe(topic, func(context.Context, any) { <-gate }, eventbus.WithBuffer(4))

	start := time.Now()
	for i := range 10_000 {
		bus.Publish(context.Background(), topic, i)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Publish bloqueó al productor: 10k publicaciones tardaron %v", elapsed)
	}
}

func TestOverflow_DropOldest(t *testing.T) {
	bus := newBus(t)
	rec := &recorder[int]{}
	started := make(chan struct{}, 16)
	gate := make(chan struct{})
	bus.Subscribe(topic, blockingHandler(rec, started, gate),
		eventbus.WithName("sub"), eventbus.WithBuffer(3), eventbus.WithOverflow(eventbus.DropOldest))

	bus.Publish(context.Background(), topic, 0)
	<-started // el 0 está en vuelo, la cola (3) queda libre
	for i := 1; i <= 5; i++ {
		bus.Publish(context.Background(), topic, i)
	}

	st := statsFor(t, bus, "sub")
	if st.Published != 6 || st.Dropped != 2 || st.QueueLen != 3 {
		t.Fatalf("stats antes de liberar = %+v", st)
	}

	close(gate)
	waitFor(t, "entrega de la cola", func() bool { return rec.len() == 4 })
	if got := rec.snapshot(); !slices.Equal(got, []int{0, 3, 4, 5}) {
		t.Fatalf("DropOldest entregó %v, se esperaba [0 3 4 5]", got)
	}
	waitFor(t, "contadores finales", func() bool { return statsFor(t, bus, "sub").Delivered == 4 })
	st = statsFor(t, bus, "sub")
	if st.Published != st.Delivered+st.Dropped+st.Panics {
		t.Fatalf("invariante de contadores rota: %+v", st)
	}
}

func TestOverflow_DropNewest(t *testing.T) {
	bus := newBus(t)
	rec := &recorder[int]{}
	started := make(chan struct{}, 16)
	gate := make(chan struct{})
	bus.Subscribe(topic, blockingHandler(rec, started, gate),
		eventbus.WithName("sub"), eventbus.WithBuffer(3), eventbus.WithOverflow(eventbus.DropNewest))

	bus.Publish(context.Background(), topic, 0)
	<-started
	for i := 1; i <= 5; i++ {
		bus.Publish(context.Background(), topic, i)
	}

	st := statsFor(t, bus, "sub")
	if st.Published != 6 || st.Dropped != 2 || st.QueueLen != 3 {
		t.Fatalf("stats antes de liberar = %+v", st)
	}

	close(gate)
	waitFor(t, "entrega de la cola", func() bool { return rec.len() == 4 })
	if got := rec.snapshot(); !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Fatalf("DropNewest entregó %v, se esperaba [0 1 2 3]", got)
	}
}

func TestOverflow_WarnIsRateLimited(t *testing.T) {
	out := captureLog(t)
	bus := newBus(t)
	started := make(chan struct{}, 16)
	gate := make(chan struct{})
	bus.Subscribe(topic, blockingHandler(&recorder[int]{}, started, gate),
		eventbus.WithName("sub"), eventbus.WithBuffer(1))

	bus.Publish(context.Background(), topic, 0)
	<-started
	for i := 1; i <= 1000; i++ {
		bus.Publish(context.Background(), topic, i)
	}
	close(gate)

	if st := statsFor(t, bus, "sub"); st.Dropped != 999 {
		t.Fatalf("Dropped = %d, se esperaba 999", st.Dropped)
	}
	if got := out.count("cola de suscripción llena"); got != 1 {
		t.Fatalf("warn de overflow registrado %d veces, se esperaba 1", got)
	}
}

func TestHandlerPanic_IsRecoveredAndWorkerContinues(t *testing.T) {
	out := captureLog(t)
	bus := newBus(t)
	rec := &recorder[int]{}
	bus.Subscribe(topic, func(_ context.Context, e any) {
		if e.(int) == 1 {
			panic("boom")
		}
		rec.add(e.(int))
	}, eventbus.WithName("sub"))

	for i := range 3 {
		bus.Publish(context.Background(), topic, i)
	}

	waitFor(t, "eventos posteriores al pánico", func() bool { return rec.len() == 2 })
	if got := rec.snapshot(); !slices.Equal(got, []int{0, 2}) {
		t.Fatalf("recibido %v, se esperaba [0 2]", got)
	}
	waitFor(t, "contadores", func() bool { return statsFor(t, bus, "sub").Delivered == 2 })
	if st := statsFor(t, bus, "sub"); st.Panics != 1 {
		t.Fatalf("Panics = %d, se esperaba 1", st.Panics)
	}
	if out.count("pánico recuperado") != 1 || out.count(topic) == 0 {
		t.Fatal("el pánico debía registrarse con el tópico")
	}
}

func TestHandlerTimeout_CancelsHandlerContext(t *testing.T) {
	bus := newBus(t)
	errs := make(chan error, 1)
	bus.Subscribe(topic, func(ctx context.Context, _ any) {
		<-ctx.Done()
		errs <- ctx.Err()
	}, eventbus.WithHandlerTimeout(30*time.Millisecond))

	bus.Publish(context.Background(), topic, 1)

	select {
	case err := <-errs:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ctx.Err() = %v, se esperaba DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el timeout del handler no se aplicó")
	}
}

func TestHandlerTimeout_UsesBusDefault(t *testing.T) {
	bus := newBus(t, eventbus.WithDefaultHandlerTimeout(time.Hour))
	deadlines := make(chan time.Time, 1)
	bus.Subscribe(topic, func(ctx context.Context, _ any) {
		d, _ := ctx.Deadline()
		deadlines <- d
	})

	before := time.Now()
	bus.Publish(context.Background(), topic, 1)

	d := <-deadlines
	if d.Before(before.Add(59*time.Minute)) || d.After(time.Now().Add(time.Hour)) {
		t.Fatalf("deadline %v no corresponde al default de 1h", d)
	}
}

func TestHandlerContext_IsDisconnectedFromPublisher(t *testing.T) {
	bus := newBus(t)
	gate := make(chan struct{})
	type result struct {
		err   error
		value any
	}
	results := make(chan result, 1)
	bus.Subscribe(topic, func(ctx context.Context, _ any) {
		<-gate
		results <- result{err: ctx.Err(), value: ctx.Value(ctxKey{})}
	})

	pubCtx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "trace-123"))
	bus.Publish(pubCtx, topic, 1)
	cancel()
	close(gate)

	r := <-results
	if r.err != nil {
		t.Fatalf("cancelar el ctx del publicador canceló el handler: %v", r.err)
	}
	if r.value != "trace-123" {
		t.Fatalf("el ctx del handler perdió los valores del publicador: %v", r.value)
	}
}

func TestPublish_NilContextIsAccepted(t *testing.T) {
	bus := newBus(t)
	got := make(chan bool, 1)
	bus.Subscribe(topic, func(ctx context.Context, _ any) { got <- ctx != nil })

	//nolint:staticcheck // se prueba explícitamente la tolerancia a ctx nil.
	bus.Publish(nil, topic, 1)

	if !<-got {
		t.Fatal("el handler recibió un ctx nil")
	}
}

func TestUnsubscribe_IsIdempotentAndConcurrencySafe(t *testing.T) {
	bus := newBus(t)
	rec := &recorder[int]{}
	unsubscribe := bus.Subscribe(topic, func(_ context.Context, e any) { rec.add(e.(int)) })

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(unsubscribe)
	}
	wg.Wait()
	unsubscribe()

	bus.Publish(context.Background(), topic, 1)

	waitFor(t, "fin del worker", func() bool { return eventbus.ActiveWorkers(bus) == 0 })
	if rec.len() != 0 || len(bus.Stats()) != 0 {
		t.Fatalf("tras unsubscribe no debía haber entregas ni stats: %v %v", rec.snapshot(), bus.Stats())
	}
}

func TestUnsubscribe_DiscardsPendingAndCancelsInFlight(t *testing.T) {
	bus := newBus(t)
	rec := &recorder[int]{}
	started := make(chan struct{}, 16)
	bus.Subscribe("keep", func(context.Context, any) {}) // otra suscripción del bus no debe verse afectada
	unsubscribe := bus.Subscribe(topic, func(ctx context.Context, e any) {
		started <- struct{}{}
		<-ctx.Done()
		rec.add(e.(int))
	}, eventbus.WithHandlerTimeout(time.Hour))

	for i := range 4 {
		bus.Publish(context.Background(), topic, i)
	}
	<-started
	unsubscribe()

	waitFor(t, "fin del worker", func() bool { return eventbus.ActiveWorkers(bus) == 1 })
	if got := rec.snapshot(); !slices.Equal(got, []int{0}) {
		t.Fatalf("solo el evento en vuelo debía completarse, recibido %v", got)
	}
}

func TestClose_DrainsPendingEvents(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	rec := &recorder[int]{}
	bus.Subscribe(topic, func(_ context.Context, e any) {
		time.Sleep(2 * time.Millisecond)
		rec.add(e.(int))
	}, eventbus.WithName("sub"))

	for i := range 20 {
		bus.Publish(context.Background(), topic, i)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bus.Close(ctx); err != nil {
		t.Fatalf("Close devolvió %v", err)
	}
	if rec.len() != 20 {
		t.Fatalf("Close no drenó: entregados %d de 20", rec.len())
	}
	if eventbus.ActiveWorkers(bus) != 0 {
		t.Fatal("quedaron workers vivos tras Close")
	}
	if err := bus.Close(ctx); err != nil {
		t.Fatalf("Close no es idempotente: %v", err)
	}
}

func TestClose_RespectsDeadline(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	started := make(chan struct{}, 16)
	bus.Subscribe(topic, func(ctx context.Context, _ any) {
		started <- struct{}{}
		<-ctx.Done() // handler cooperativo: termina cuando el bus lo cancela
	}, eventbus.WithName("sub"), eventbus.WithHandlerTimeout(time.Hour))

	for i := range 3 {
		bus.Publish(context.Background(), topic, i)
	}
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := bus.Close(ctx)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close devolvió %v, se esperaba DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Close no respetó el deadline: %v", elapsed)
	}
	waitFor(t, "fin de workers", func() bool { return eventbus.ActiveWorkers(bus) == 0 })
	st := statsFor(t, bus, "sub")
	if st.Delivered != 1 || st.Dropped != 2 || st.QueueLen != 0 {
		t.Fatalf("stats tras deadline = %+v", st)
	}
}

func TestPublishAndSubscribeAfterClose(t *testing.T) {
	out := captureLog(t)
	bus := eventbus.NewInMemory(nil)
	rec := &recorder[int]{}
	bus.Subscribe(topic, func(_ context.Context, e any) { rec.add(e.(int)) }, eventbus.WithName("sub"))
	if err := bus.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		bus.Publish(context.Background(), topic, i)
	}
	unsubscribe := bus.Subscribe(topic, func(context.Context, any) {})
	unsubscribe()

	if rec.len() != 0 {
		t.Fatalf("no debía entregarse nada tras Close: %v", rec.snapshot())
	}
	if st := statsFor(t, bus, "sub"); st.Published != 0 {
		t.Fatalf("Published = %d tras Close", st.Published)
	}
	if got := out.count("el bus está cerrado"); got != 2 {
		t.Fatalf("se esperaban 2 warns (publicación rate-limited + suscripción), hubo %d", got)
	}
}

func TestSubscribe_NilHandlerIsIgnored(t *testing.T) {
	bus := newBus(t)
	unsubscribe := bus.Subscribe(topic, nil)
	unsubscribe()
	if len(bus.Stats()) != 0 || eventbus.ActiveWorkers(bus) != 0 {
		t.Fatal("un handler nil no debía registrar suscripción")
	}
}

func TestOptions_InvalidValuesKeepDefaults(t *testing.T) {
	bus := newBus(t, eventbus.WithDefaultBuffer(2), eventbus.WithDefaultBuffer(0), eventbus.WithDefaultHandlerTimeout(-1))
	started := make(chan struct{}, 16)
	gate := make(chan struct{})
	defer close(gate)
	bus.Subscribe(topic, blockingHandler(&recorder[int]{}, started, gate),
		eventbus.WithName(""), eventbus.WithBuffer(0), eventbus.WithOverflow(eventbus.OverflowPolicy(99)),
		eventbus.WithHandlerTimeout(0))

	bus.Publish(context.Background(), topic, 0)
	<-started
	for i := 1; i <= 4; i++ {
		bus.Publish(context.Background(), topic, i)
	}

	st := bus.Stats()[0]
	if st.Name != topic+"#1" {
		t.Errorf("nombre por defecto = %q", st.Name)
	}
	if st.QueueLen != 2 || st.Dropped != 2 {
		t.Errorf("se esperaba el buffer por defecto del bus (2): %+v", st)
	}
}

func TestStats_AreSortedByTopicAndName(t *testing.T) {
	bus := newBus(t)
	bus.Subscribe("b", func(context.Context, any) {}, eventbus.WithName("z"))
	bus.Subscribe("a", func(context.Context, any) {}, eventbus.WithName("y"))
	bus.Subscribe("b", func(context.Context, any) {}, eventbus.WithName("x"))

	var got []string
	for _, s := range bus.Stats() {
		got = append(got, s.Topic+"/"+s.Name)
	}
	if !slices.Equal(got, []string{"a/y", "b/x", "b/z"}) {
		t.Fatalf("orden de Stats = %v", got)
	}
}

func TestOverflowPolicy_String(t *testing.T) {
	cases := map[eventbus.OverflowPolicy]string{
		eventbus.DropOldest:         "drop_oldest",
		eventbus.DropNewest:         "drop_newest",
		eventbus.OverflowPolicy(42): "unknown",
	}
	for policy, want := range cases {
		if got := policy.String(); got != want {
			t.Errorf("%d.String() = %q, se esperaba %q", policy, got, want)
		}
	}
}

func TestConcurrency_PublishersBySubscribersWithoutLoss(t *testing.T) {
	const publishers, subscribers, perPublisher = 8, 4, 500
	type msg struct{ pub, seq int }

	bus := newBus(t)
	recs := make([]*recorder[msg], subscribers)
	for i := range recs {
		rec := &recorder[msg]{}
		recs[i] = rec
		bus.Subscribe(topic, func(_ context.Context, e any) { rec.add(e.(msg)) },
			eventbus.WithBuffer(publishers*perPublisher))
	}

	var wg sync.WaitGroup
	for p := range publishers {
		wg.Go(func() {
			for s := range perPublisher {
				bus.Publish(context.Background(), topic, msg{pub: p, seq: s})
			}
		})
	}
	wg.Wait()

	for i, rec := range recs {
		waitFor(t, "todas las entregas", func() bool { return rec.len() == publishers*perPublisher })
		last := make(map[int]int)
		for _, m := range rec.snapshot() {
			if prev, ok := last[m.pub]; ok && m.seq != prev+1 {
				t.Fatalf("suscriptor %d: orden roto para publicador %d (%d tras %d)", i, m.pub, m.seq, prev)
			}
			last[m.pub] = m.seq
		}
	}
	for _, st := range bus.Stats() {
		if st.Dropped != 0 || st.Panics != 0 {
			t.Fatalf("hubo pérdidas con capacidad suficiente: %+v", st)
		}
	}
}

func TestNoGoroutineLeaks(t *testing.T) {
	baseline := runtime.NumGoroutine()

	bus := eventbus.NewInMemory(nil)
	var unsubs []func()
	for i := range 10 {
		unsubs = append(unsubs, bus.Subscribe(topic, func(context.Context, any) {}, eventbus.WithName(string(rune('a'+i)))))
	}
	for i := range 100 {
		bus.Publish(context.Background(), topic, i)
	}
	unsubs[0]()
	unsubs[1]()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bus.Close(ctx); err != nil {
		t.Fatal(err)
	}

	if eventbus.ActiveWorkers(bus) != 0 {
		t.Fatalf("workers vivos tras Close: %d", eventbus.ActiveWorkers(bus))
	}
	waitFor(t, "goroutines liberadas", func() bool { return runtime.NumGoroutine() <= baseline+1 })
}
