package scheduling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeRecalculator struct {
	mu    sync.Mutex
	calls []time.Time
}

func (f *fakeRecalculator) RecalculateDue(_ context.Context, now time.Time) ([]services.PointRecalculation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, now)
	return nil, nil
}

func TestNextRun(t *testing.T) {
	s := NewRecalcScheduler(&fakeRecalculator{}, logger.New("BaselineTest", "Info"), 10)
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		now  time.Time
		want time.Time
	}{
		{base.Add(5 * time.Minute), base.Add(10 * time.Minute)},
		{base.Add(10 * time.Minute), base.Add(70 * time.Minute)},
		{base.Add(30 * time.Minute), base.Add(70 * time.Minute)},
	}

	for _, tt := range tests {
		if got := s.nextRun(tt.now); !got.Equal(tt.want) {
			t.Fatalf("nextRun(%v) = %v, se esperaba %v", tt.now, got, tt.want)
		}
	}
}

func TestStartRunsOnceAtBoot(t *testing.T) {
	fake := &fakeRecalculator{}
	s := NewRecalcScheduler(fake, logger.New("BaselineTest", "Info"), 10)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		fake.mu.Lock()
		n := len(fake.calls)
		fake.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no se ejecutó al arrancar")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start no terminó al cancelar el contexto")
	}
}
