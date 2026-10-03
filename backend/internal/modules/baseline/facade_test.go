package baseline

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

func TestToEffectiveBaseline(t *testing.T) {
	first := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	t.Run("sin versión ⇒ NONE con first_seen_at", func(t *testing.T) {
		out := toEffectiveBaseline(7, services.EffectiveResult{FirstSeenAt: &first})
		if out.Status != StatusNone || out.Signal != nil || out.DeviceFirstSeenAt == nil || out.MeasuringPointID != 7 {
			t.Fatalf("out = %+v", out)
		}
	})

	t.Run("mapeo completo de señal, componentes y flags", func(t *testing.T) {
		p90 := 0.8
		validFrom := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
		v := &entities.BaselineVersion{
			ID: 10, Version: 3, ValidFrom: validFrom, Confidence: entities.ConfidenceLow, AlgorithmVersion: "1.0.0",
			Signal: entities.SignalBaseline{MedianB2: 54, ThetaD: 67, P90Planitud: &p90, DeltaMin: 1},
			Components: []entities.ComponentStat{
				{WindowHours: 6, Component: "D", Median: 8, MAD: 2, NWindows: 700},
				{WindowHours: 24, Component: "C", Median: 0.2, MAD: 0.05, NWindows: 340},
			},
			Flags: []entities.Flag{entities.FlagBaselineReview, entities.FlagSignalDegenerate},
		}

		out := toEffectiveBaseline(7, services.EffectiveResult{Version: v, FirstSeenAt: &first})
		if out.Status != StatusAvailable || out.VersionID != 10 || out.Version != 3 || out.Confidence != "LOW" ||
			!out.ValidFrom.Equal(validFrom) || out.AlgorithmVersion != "1.0.0" {
			t.Fatalf("out = %+v", out)
		}
		if out.Signal.ThetaD != 67 || out.Signal.MedianB2 != 54 || *out.Signal.P90Planitud != 0.8 || out.Signal.MinElevatedB2 != nil {
			t.Fatalf("signal = %+v", out.Signal)
		}
		if len(out.Components) != 2 || out.Components[1].Component != "C" || out.Components[1].NWindows != 340 {
			t.Fatalf("components = %+v", out.Components)
		}
		if len(out.Flags) != 2 || out.Flags[0] != "SIGNAL_DEGENERATE" || out.Flags[1] != "BASELINE_REVIEW" {
			t.Fatalf("flags = %v", out.Flags)
		}
		// La fachada no expone punteros internos de la entidad.
		*out.Signal.P90Planitud = 99
		if p90 != 0.8 {
			t.Fatal("la fachada comparte memoria con la entidad")
		}
	})
}
