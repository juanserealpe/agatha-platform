package scheduling

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeCatchUp struct{ calls atomic.Int32 }

func (f *fakeCatchUp) CatchUp(context.Context) ([]services.PointReplay, error) {
	f.calls.Add(1)
	return []services.PointReplay{{PointID: 11556, Hours: 1}}, nil
}

func log() *logger.Logger { return logger.New("AlertManagementTest", "Error") }

func TestCatchUpScheduler_NextRunAfterRuleEngine(t *testing.T) {
	s := NewCatchUpScheduler(&fakeCatchUp{}, log(), DefaultCatchUpMinute)
	base := time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)
	for now, want := range map[time.Time]time.Time{
		base.Add(50 * time.Minute): base.Add(55 * time.Minute),
		base.Add(55 * time.Minute): base.Add(115 * time.Minute),
	} {
		if got := s.nextRun(now); !got.Equal(want) {
			t.Fatalf("nextRun(%v) = %v, se esperaba %v", now, got, want)
		}
	}
	if NewCatchUpScheduler(&fakeCatchUp{}, log(), 99).tickMinute != DefaultCatchUpMinute {
		t.Fatal("un minuto inválido usa 55")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			t.Fatal("timeout")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestCatchUpScheduler_RunsAtStartupAndStopsOnCancel(t *testing.T) {
	catchUp := &fakeCatchUp{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { NewCatchUpScheduler(catchUp, log(), 55).Start(ctx); close(done) }()

	waitFor(t, func() bool { return catchUp.calls.Load() == 1 })
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("no terminó al cancelar")
	}
}
