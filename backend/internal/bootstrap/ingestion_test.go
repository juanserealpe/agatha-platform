package bootstrap

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestCatchUpSettings(t *testing.T) {
	log := logger.New("test", "Info")

	t.Run("sin callback (PC): DataOS es la única vía, cada 15 min y hasta ahora", func(t *testing.T) {
		t.Setenv(CatchUpLookbackEnv, "")
		s := catchUpSettings(log, false)
		if s.interval != 15*time.Minute || s.cfg.MinAge != 0 {
			t.Fatalf("settings = %+v", s)
		}
		if s.cfg.Lookback != 7*24*time.Hour {
			t.Fatalf("lookback = %s", s.cfg.Lookback)
		}
	})

	t.Run("con callback (VPS): respaldo horario que no toca la última hora", func(t *testing.T) {
		s := catchUpSettings(log, true)
		if s.interval != time.Hour || s.cfg.MinAge != time.Hour {
			t.Fatalf("settings = %+v", s)
		}
	})

	t.Run("la ventana se configura por entorno", func(t *testing.T) {
		t.Setenv(CatchUpLookbackEnv, "72h")
		if s := catchUpSettings(log, true); s.cfg.Lookback != 72*time.Hour {
			t.Fatalf("lookback = %s", s.cfg.Lookback)
		}
	})
}
