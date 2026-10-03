package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

func TestHistoricalWindowGenerator(t *testing.T) {
	gen := HistoricalWindowGenerator{}
	from := time.Date(2026, 4, 1, 1, 30, 0, 0, time.UTC)
	to := time.Date(2026, 4, 3, 2, 0, 0, 0, time.UTC)

	t.Run("6 h con fin cada 3 h", func(t *testing.T) {
		ends := gen.Ends(from, to, HistoricalWindows[0])
		// Primer fin ≥ 07:30 alineado a 3 h ⇒ 09:00 del día 1; último ≤ 02:00 del día 3 ⇒ 00:00.
		if !ends[0].Equal(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)) {
			t.Fatalf("primer fin = %v", ends[0])
		}
		if last := ends[len(ends)-1]; !last.Equal(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("último fin = %v", last)
		}
		for _, e := range ends {
			if e.Hour()%3 != 0 || e.Minute() != 0 || e.Add(-6*time.Hour).Before(from) || e.After(to) {
				t.Fatalf("fin fuera de regla: %v", e)
			}
		}
	})

	t.Run("24 h con fin cada 6 h", func(t *testing.T) {
		ends := gen.Ends(from, to, HistoricalWindows[1])
		// Primer fin ≥ 01:30 del día 2 ⇒ 06:00; último 00:00 del día 3.
		want := []time.Time{
			time.Date(2026, 4, 2, 6, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 18, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		}
		if len(ends) != len(want) {
			t.Fatalf("ends = %v", ends)
		}
		for i := range want {
			if !ends[i].Equal(want[i]) {
				t.Fatalf("ends[%d] = %v, se esperaba %v", i, ends[i], want[i])
			}
		}
	})

	t.Run("rango menor que la ventana no produce fines", func(t *testing.T) {
		if ends := gen.Ends(from, from.Add(5*time.Hour), HistoricalWindows[0]); len(ends) != 0 {
			t.Fatalf("ends = %v", ends)
		}
	})
}

func statByKey(stats []entities.ComponentStat, hours int, component string) (entities.ComponentStat, bool) {
	for _, st := range stats {
		if st.WindowHours == hours && st.Component == component {
			return st, true
		}
	}
	return entities.ComponentStat{}, false
}

func TestComponentBaselineCalculator(t *testing.T) {
	cfg := DefaultConfig()
	calc := NewComponentBaselineCalculator(cfg)
	sig := features.Signal{MedianB2: 55, ThetaD: 67, MinElevatedB2: 68, HasMinElevated: true, DeltaMin: 1}
	none := entities.ExclusionSet{}

	t.Run("C solo en 24 h y conteo de ventanas", func(t *testing.T) {
		// 30 días: 24 h ⇒ fines desde día 1 00:00 hasta día 30 00:00 cada 6 h = 117.
		samples := synthetic(day0, 30)
		res := calc.Calculate(samples, sig, none, day0, day0.AddDate(0, 0, 30))
		if res.Windows24h != 117 {
			t.Fatalf("Windows24h = %d, se esperaba 117", res.Windows24h)
		}
		if _, ok := statByKey(res.Stats, 6, features.ComponentC); ok {
			t.Fatal("no debe haber C en 6 h")
		}
		if st, ok := statByKey(res.Stats, 24, features.ComponentC); !ok || st.NWindows != 117 {
			t.Fatalf("C 24 h = %+v", st)
		}
		if !HasCompleteStats(res.Stats, 24) || !HasCompleteStats(res.Stats, 6) {
			t.Fatal("faltan combinaciones")
		}
	})

	t.Run("descarta ventanas que solapan la exclusión", func(t *testing.T) {
		samples := synthetic(day0, 30)
		to := day0.AddDate(0, 0, 30)
		excl := entities.ExclusionSet{Periods: []entities.ExcludedPeriod{
			{From: day0.AddDate(0, 0, 10), To: day0.AddDate(0, 0, 11)},
		}}
		res := calc.Calculate(samples, sig, excl, day0, to)
		// Ventanas de 24 h que solapan [día 10, día 11): fines en (día 10, día 12) ⇒ 7.
		if res.Windows24h != 117-7 {
			t.Fatalf("Windows24h = %d, se esperaba %d", res.Windows24h, 117-7)
		}
	})

	t.Run("descarta ventanas con cobertura < 70 %", func(t *testing.T) {
		samples := synthetic(day0, 30)
		// Quita B2 del día 5 completo.
		for i := range samples {
			if samples[i].T.Day() == day0.AddDate(0, 0, 5).Day() && samples[i].T.Month() == day0.Month() {
				samples[i].HasB2 = false
			}
		}
		res := calc.Calculate(samples, sig, none, day0, day0.AddDate(0, 0, 30))
		if res.Windows24h >= 117 {
			t.Fatalf("Windows24h = %d, se esperaban menos de 117", res.Windows24h)
		}
	})

	windowsCase := func(days int) ComponentCalculation {
		cfg := DefaultConfig()
		return NewComponentBaselineCalculator(cfg).Calculate(synthetic(day0, days), sig, none, day0, day0.AddDate(0, 0, days))
	}

	t.Run("19 ventanas de 24 h no se guardan", func(t *testing.T) {
		// días = 5,5 no es entero; 5 días ⇒ 17 ventanas, 6 días ⇒ 21. Se recorta el rango.
		samples := synthetic(day0, 6)
		to := day0.Add(5*24*time.Hour + 12*time.Hour) // fines 24 h: día1 00:00 … día5 12:00 = 19
		res := calc.Calculate(samples, sig, none, day0, to)
		if res.Windows24h != 19 {
			t.Fatalf("Windows24h = %d, se esperaba 19", res.Windows24h)
		}
		if HasCompleteStats(res.Stats, 24) {
			t.Fatal("con 19 ventanas no debe guardarse 24 h")
		}
	})

	t.Run("20 ventanas ⇒ LOW", func(t *testing.T) {
		samples := synthetic(day0, 6)
		to := day0.Add(5*24*time.Hour + 18*time.Hour) // 20 fines de 24 h
		res := calc.Calculate(samples, sig, none, day0, to)
		if res.Windows24h != 20 || !HasCompleteStats(res.Stats, 24) {
			t.Fatalf("Windows24h = %d", res.Windows24h)
		}
		if got := NewConfidencePolicy(cfg).Evaluate(res.Stats); got != entities.ConfidenceLow {
			t.Fatalf("confianza = %s, se esperaba LOW", got)
		}
	})

	t.Run("60 ventanas ⇒ NORMAL", func(t *testing.T) {
		// 24 h: fines día1 00:00 … ⇒ 60 fines con (60−1)·6 h = 354 h tras día 1.
		res := windowsCase(16)
		if res.Windows24h < 60 {
			t.Fatalf("Windows24h = %d", res.Windows24h)
		}
		if got := NewConfidencePolicy(cfg).Evaluate(res.Stats); got != entities.ConfidenceNormal {
			t.Fatalf("confianza = %s, se esperaba NORMAL (24h=%d, 6h=%d)", got, res.Windows24h, res.Windows6h)
		}
	})
}

func TestConfidencePolicyBoundaries(t *testing.T) {
	policy := NewConfidencePolicy(DefaultConfig())
	full := func(n int) []entities.ComponentStat {
		out := []entities.ComponentStat{}
		for _, spec := range HistoricalWindows {
			for _, c := range componentsByWindow[spec.Hours] {
				out = append(out, entities.ComponentStat{WindowHours: spec.Hours, Component: c, NWindows: n})
			}
		}
		return out
	}

	if got := policy.Evaluate(full(60)); got != entities.ConfidenceNormal {
		t.Fatalf("60 ⇒ %s", got)
	}
	if got := policy.Evaluate(full(59)); got != entities.ConfidenceLow {
		t.Fatalf("59 ⇒ %s", got)
	}
	if got := policy.Evaluate(full(60)[:6]); got != entities.ConfidenceLow {
		t.Fatalf("sin C ⇒ %s", got)
	}
	if got := policy.Evaluate(nil); got != entities.ConfidenceLow {
		t.Fatalf("vacío ⇒ %s", got)
	}
}
