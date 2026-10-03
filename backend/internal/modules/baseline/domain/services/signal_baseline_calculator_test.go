package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

func b2Samples(values []float64, withPl bool) []features.Sample {
	out := make([]features.Sample, len(values))
	for i, v := range values {
		out[i] = features.Sample{T: day0.Add(time.Duration(i) * features.Slot), B2: v, HasB2: true}
		if withPl {
			out[i].Pl, out[i].HasPl = float64(i), true
		}
	}
	return out
}

// repeatCycle repite 0..9 + offset hasta n valores.
func repeatCycle(n int, offset float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i%10) + offset
	}
	return out
}

func TestSignalBaselineCalculator(t *testing.T) {
	calc := NewSignalBaselineCalculator(DefaultConfig())

	t.Run("valores conocidos", func(t *testing.T) {
		// 100 valores: 10 de cada uno de 50..59. Mediana 54,5; P90 tipo 7:
		// h = 99 × 0,9 = 89,1 ⇒ 58 + 0,1 × (59 − 58) = 58,1.
		res := calc.Calculate(b2Samples(repeatCycle(100, 50), true))
		if !res.OK {
			t.Fatal("OK = false")
		}
		sig := res.Signal
		if sig.MedianB2 != 54.5 || !approxEq(sig.ThetaD, 58.1) || sig.DeltaMin != 1 || sig.NRecords != 100 {
			t.Fatalf("signal = %+v", sig)
		}
		if sig.P90Planitud == nil || *sig.P90Planitud != features.Quantile(repeatPl(100), 0.9) {
			t.Fatalf("P90Planitud = %v", sig.P90Planitud)
		}
		if sig.MinElevatedB2 == nil || *sig.MinElevatedB2 != 59 || len(res.Flags) != 0 {
			t.Fatalf("MinElevatedB2 = %v, flags %v", sig.MinElevatedB2, res.Flags)
		}
	})

	t.Run("degenerada: ningún B2 > θ_D", func(t *testing.T) {
		// 80 valores 50 y 20 valores 60: P90 = 60 y nada lo supera.
		vals := make([]float64, 100)
		for i := range vals {
			vals[i] = 50
			if i >= 80 {
				vals[i] = 60
			}
		}
		res := calc.Calculate(b2Samples(vals, false))
		if res.Signal.ThetaD != 60 || res.Signal.MinElevatedB2 != nil ||
			!entities.HasFlag(res.Flags, entities.FlagSignalDegenerate) {
			t.Fatalf("signal = %+v, flags %v", res.Signal, res.Flags)
		}
	})

	t.Run("min elevado sobre θ_D", func(t *testing.T) {
		vals := repeatCycle(100, 50)
		vals[0] = 70
		vals[1] = 65
		res := calc.Calculate(b2Samples(vals, false))
		if res.Signal.MinElevatedB2 == nil || *res.Signal.MinElevatedB2 != 65 {
			t.Fatalf("MinElevatedB2 = %v, se esperaba 65", res.Signal.MinElevatedB2)
		}
	})

	t.Run("menos de 96 registros", func(t *testing.T) {
		if res := calc.Calculate(b2Samples(repeatCycle(95, 50), false)); res.OK {
			t.Fatal("OK = true con 95 registros")
		}
	})

	t.Run("sin Planitud", func(t *testing.T) {
		res := calc.Calculate(b2Samples(repeatCycle(96, 50), false))
		if !res.OK || res.Signal.P90Planitud != nil || res.Signal.NPlanitudRecords != 0 {
			t.Fatalf("signal = %+v", res.Signal)
		}
	})

	t.Run("baja variabilidad", func(t *testing.T) {
		vals := make([]float64, 96)
		for i := range vals {
			vals[i] = 50
		}
		vals[0] = 80
		res := calc.Calculate(b2Samples(vals, false))
		if !entities.HasFlag(res.Flags, entities.FlagSignalLowVariability) {
			t.Fatalf("se esperaba SIGNAL_LOW_VARIABILITY, flags %v", res.Flags)
		}
		if res.Signal.DeltaMin != 30 {
			t.Fatalf("DeltaMin = %v, se esperaba 30", res.Signal.DeltaMin)
		}
	})
}

func repeatPl(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i)
	}
	return out
}
