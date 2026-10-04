package scheduling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeReconciler struct {
	mu    sync.Mutex
	calls []time.Time
}

func (f *fakeReconciler) Reconcile(_ context.Context, now time.Time) ([]services.PointReconciliation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, now)
	return nil, nil
}

func TestNextRun(t *testing.T) {
	s := NewHourlyScheduler(&fakeReconciler{}, logger.New("ProcessingTest", "Error"), 35)
	base := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		now  time.Time
		want time.Time
	}{
		{base.Add(5 * time.Minute), base.Add(35 * time.Minute)},
		{base.Add(35 * time.Minute), base.Add(95 * time.Minute)},
		{base.Add(50 * time.Minute), base.Add(95 * time.Minute)},
	}

	for _, tt := range tests {
		if got := s.nextRun(tt.now); !got.Equal(tt.want) {
			t.Fatalf("nextRun(%v) = %v, se esperaba %v", tt.now, got, tt.want)
		}
	}
}

func TestStartRunsOnceAtStartupAndStopsOnCancel(t *testing.T) {
	reconciler := &fakeReconciler{}
	s := NewHourlyScheduler(reconciler, logger.New("ProcessingTest", "Error"), 35)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		reconciler.mu.Lock()
		n := len(reconciler.calls)
		reconciler.mu.Unlock()
		if n == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("no se ejecutó la reconciliación inicial")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start no terminó al cancelar el contexto")
	}
}
