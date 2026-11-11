package scheduling

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeRecalculator struct {
	mu      sync.Mutex
	calls   []time.Time
	summary []services.PointRecalculation
	err     error
	called  chan struct{}
}

func (f *fakeRecalculator) RecalculateDue(_ context.Context, now time.Time) ([]services.PointRecalculation, error) {
	f.mu.Lock()
	f.calls = append(f.calls, now)
	f.mu.Unlock()
	if f.called != nil {
		select {
		case f.called <- struct{}{}:
		default:
		}
	}
	return f.summary, f.err
}

func testLogger() *logger.Logger { return logger.New("BaselineSchedulerTest", "Error") }

func TestNewRecalcScheduler_InvalidMinuteFallsBackTo10(t *testing.T) {
	for _, minute := range []int{-1, 60} {
		if s := NewRecalcScheduler(&fakeRecalculator{}, testLogger(), minute); s.tickMinute != 10 {
			t.Fatalf("minuto %d → %d, se esperaba 10", minute, s.tickMinute)
		}
	}
	if s := NewRecalcScheduler(&fakeRecalculator{}, testLogger(), 0); s.tickMinute != 0 {
		t.Fatalf("el minuto 0 es válido, quedó %d", s.tickMinute)
	}
}

func TestNextRun(t *testing.T) {
	s := NewRecalcScheduler(&fakeRecalculator{}, testLogger(), 10)
	base := time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		now, want time.Time
	}{
		{base.Add(5 * time.Minute), base.Add(10 * time.Minute)},                                    // 14:05 → 14:10
		{base.Add(10 * time.Minute), base.Add(70 * time.Minute)},                                   // 14:10 exacto → 15:10
		{base.Add(45 * time.Minute), base.Add(70 * time.Minute)},                                   // 14:45 → 15:10
		{base.Add(5 * time.Minute).In(time.FixedZone("COT", -5*3600)), base.Add(10 * time.Minute)}, // la zona no cambia el instante
	}
	for _, tt := range tests {
		if got := s.nextRun(tt.now); !got.Equal(tt.want) {
			t.Errorf("nextRun(%v) = %v, se esperaba %v", tt.now, got, tt.want)
		}
	}
}

func TestStart_RunsOnceAtStartupAndStopsOnCancel(t *testing.T) {
	// Un reloj en el pasado haría vencer el próximo tick al instante: se usa la hora real en COT.
	fixed := time.Now().In(time.FixedZone("COT", -5*3600))
	rec := &fakeRecalculator{
		called: make(chan struct{}, 1),
		summary: []services.PointRecalculation{
			{PointID: 1, Due: true, Result: &services.RecalculationResult{Created: true}},
			{PointID: 2, Err: errors.New("falló")},
		},
	}
	s := NewRecalcScheduler(rec, testLogger(), 10)
	s.now = func() time.Time { return fixed }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()

	select {
	case <-rec.called:
	case <-time.After(2 * time.Second):
		t.Fatal("Start no ejecutó el recálculo al arrancar")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start no terminó al cancelar el contexto")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.calls) != 1 || !rec.calls[0].Equal(fixed) || rec.calls[0].Location() != time.UTC {
		t.Fatalf("llamadas = %v, se esperaba una en UTC", rec.calls)
	}
}

func TestRun_SkipsWhenCanceledAndSurvivesErrors(t *testing.T) {
	rec := &fakeRecalculator{err: errors.New("db caída")}
	s := NewRecalcScheduler(rec, testLogger(), 10)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	s.run(canceled)
	if len(rec.calls) != 0 {
		t.Fatal("con el contexto cancelado no se recalcula")
	}

	s.run(context.Background()) // el error se registra y no entra en pánico
	if len(rec.calls) != 1 {
		t.Fatalf("llamadas = %d, se esperaba 1", len(rec.calls))
	}
}
