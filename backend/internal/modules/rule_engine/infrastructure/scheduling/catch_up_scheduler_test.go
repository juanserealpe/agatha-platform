package scheduling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeCatchUp struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeCatchUp) CatchUp(context.Context) ([]services.PointCatchUp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return []services.PointCatchUp{{PointID: 11556, Hours: 1, Saved: 1, Published: 1}}, nil
}

func (f *fakeCatchUp) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestNextRun(t *testing.T) {
	s := NewCatchUpScheduler(&fakeCatchUp{}, logger.New("RuleEngineTest", "Error"), 50)
	base := time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		now  time.Time
		want time.Time
	}{
		{base.Add(35 * time.Minute), base.Add(50 * time.Minute)},
		{base.Add(50 * time.Minute), base.Add(110 * time.Minute)},
		{base.Add(55 * time.Minute), base.Add(110 * time.Minute)},
	}

	for _, tt := range tests {
		if got := s.nextRun(tt.now); !got.Equal(tt.want) {
			t.Fatalf("nextRun(%v) = %v, se esperaba %v", tt.now, got, tt.want)
		}
	}
}

func TestNewCatchUpScheduler_InvalidMinuteFallsBackToDefault(t *testing.T) {
	if s := NewCatchUpScheduler(&fakeCatchUp{}, logger.New("RuleEngineTest", "Error"), 75); s.tickMinute != DefaultCatchUpMinute {
		t.Fatalf("tickMinute = %d, se esperaba %d", s.tickMinute, DefaultCatchUpMinute)
	}
}

func TestStartRunsOnceAtStartupAndStopsOnCancel(t *testing.T) {
	catchUp := &fakeCatchUp{}
	s := NewCatchUpScheduler(catchUp, logger.New("RuleEngineTest", "Error"), 50)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for catchUp.count() != 1 {
		select {
		case <-deadline:
			t.Fatal("no se ejecutó el catch-up inicial")
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
