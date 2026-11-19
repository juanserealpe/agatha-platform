package scheduling

import (
	"context"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeMotionRunner struct{ calls int }

func (f *fakeMotionRunner) RunLive(context.Context) ([]services.MotionPointRun, error) {
	f.calls++
	return []services.MotionPointRun{{PointID: 11556, Saved: 1, Published: 1}, {PointID: 7, WithoutHistory: true}}, nil
}

func TestMotionScheduler_NextTickAtMinute40(t *testing.T) {
	base := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ now, want time.Time }{
		{base.Add(10 * time.Minute), base.Add(40 * time.Minute)},
		{base.Add(40 * time.Minute), base.Add(100 * time.Minute)},
		{base.Add(59 * time.Minute), base.Add(100 * time.Minute)},
	} {
		if got := nextTick(tc.now, DefaultMotionMinute); !got.Equal(tc.want) {
			t.Fatalf("nextTick(%v) = %v, se esperaba %v", tc.now, got, tc.want)
		}
	}
}

func TestMotionScheduler_RunsOnStartAndStops(t *testing.T) {
	runner := &fakeMotionRunner{}
	s := NewMotionScheduler(runner, logger.New("RuleEngineTest", "Error"), 99) // inválido ⇒ 40
	if s.tickMinute != DefaultMotionMinute {
		t.Fatalf("minuto = %d", s.tickMinute)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Start(ctx); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
	if runner.calls != 1 {
		t.Fatalf("corridas = %d, se esperaba 1 al arrancar", runner.calls)
	}
}
