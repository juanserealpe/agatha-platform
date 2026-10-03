package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// scanFixture: 30 días con un episodio de 12 h el día 15 y su línea base sin exclusión.
func scanFixture(t *testing.T) ([]features.Sample, features.Signal, features.ComponentStats, episode, time.Time, time.Time) {
	t.Helper()

	from, to := day0, day0.AddDate(0, 0, 30)
	ep := episode{from: day0.AddDate(0, 0, 15), to: day0.AddDate(0, 0, 15).Add(12 * time.Hour)}
	samples := synthetic(from, 30, ep)

	est := NewRobustBaselineEstimator(DefaultConfig())
	base := est.calculate(samples, entities.ExclusionSet{}, from, to)
	if !base.signal.OK || !HasCompleteStats(base.components.Stats, 24) {
		t.Fatal("la línea base del fixture no es completa")
	}

	return samples, base.signal.Signal.ToFeatureSignal(), entities.FeatureStats(base.components.Stats, 24), ep, from, to
}

func TestHourlyAnomalyScanner(t *testing.T) {
	samples, sig, stats, ep, from, to := scanFixture(t)
	scanner := NewHourlyAnomalyScanner(DefaultConfig())

	// Con ruido uniforme alguna ventana aislada puede cruzar 0,18: no se exige
	// "cero marcas fuera", sino que el episodio domine.
	t.Run("marca el episodio sintético con el I máximo", func(t *testing.T) {
		marks := scanner.Scan(samples, sig, stats, from, to)

		touches := func(m HourMark) bool { return m.T.After(ep.from) && !m.T.Add(-24*time.Hour).After(ep.to) }

		var peak HourMark
		inEpisode := 0
		for _, m := range marks {
			if m.T.Minute() != 0 || m.T.Before(from.Add(24*time.Hour)) || m.T.After(to) {
				t.Fatalf("marca fuera de la grilla horaria: %v", m.T)
			}
			if touches(m) {
				inEpisode++
			}
			if m.I > peak.I {
				peak = m
			}
		}

		if inEpisode < 12 {
			t.Fatalf("solo %d horas marcadas en el episodio", inEpisode)
		}
		if !touches(peak) || peak.I < 0.5 {
			t.Fatalf("pico fuera del episodio o bajo: %v (I = %v)", peak.T, peak.I)
		}
	})

	t.Run("ventana sin C no se marca", func(t *testing.T) {
		noPl := make([]features.Sample, len(samples))
		copy(noPl, samples)
		for i := range noPl {
			noPl[i].HasPl = false
		}
		if marks := scanner.Scan(noPl, sig, stats, from, to); len(marks) != 0 {
			t.Fatalf("se marcaron %d horas sin C", len(marks))
		}
	})

	t.Run("I igual al corte se marca", func(t *testing.T) {
		marks := scanner.Scan(samples, sig, stats, from, to)
		peak := marks[0]
		for _, m := range marks {
			if m.I > peak.I {
				peak = m
			}
		}

		cfg := DefaultConfig()
		cfg.ExclusionICutoff = peak.I
		exact := NewHourlyAnomalyScanner(cfg).Scan(samples, sig, stats, from, to)
		found := false
		for _, m := range exact {
			if m.T.Equal(peak.T) {
				found = true
			}
		}
		if !found {
			t.Fatalf("la hora con I = corte (%v) no se marcó", peak.I)
		}
	})
}

func TestExclusionPolicy(t *testing.T) {
	policy := NewExclusionPolicy(DefaultConfig())
	from := day0
	to := day0.AddDate(0, 0, 10) // 240 h ⇒ tope 96 h
	h := func(d, hour int) time.Time { return day0.AddDate(0, 0, d).Add(time.Duration(hour) * time.Hour) }

	t.Run("une los solapes y conserva el pico", func(t *testing.T) {
		set := policy.Apply([]HourMark{{T: h(3, 0), I: 0.3}, {T: h(3, 1), I: 0.5}}, from, to)
		if len(set.Periods) != 1 {
			t.Fatalf("periods = %+v", set.Periods)
		}
		p := set.Periods[0]
		if !p.From.Equal(h(2, 0)) || !p.To.Equal(h(3, 1)) || p.PeakI != 0.5 {
			t.Fatalf("period = %+v", p)
		}
		if set.CapReached || len(set.MarkedHours) != 2 || set.Duration() != 25*time.Hour {
			t.Fatalf("set = %+v", set)
		}
	})

	t.Run("intervalos contiguos se fusionan", func(t *testing.T) {
		set := policy.Apply([]HourMark{{T: h(3, 0), I: 0.3}, {T: h(4, 0), I: 0.3}}, from, to)
		if len(set.Periods) != 1 || set.Duration() != 48*time.Hour {
			t.Fatalf("periods = %+v", set.Periods)
		}
	})

	t.Run("tope 40 % por I descendente con desempate por t", func(t *testing.T) {
		marks := []HourMark{
			{T: h(9, 0), I: 0.5}, // empate, más tardía ⇒ queda afuera
			{T: h(2, 0), I: 0.9},
			{T: h(4, 0), I: 0.8},
			{T: h(6, 0), I: 0.7},
			{T: h(8, 0), I: 0.5}, // empate, más temprana ⇒ entra (cuarta: 96 h exactas)
		}
		set := policy.Apply(marks, from, to)
		if !set.CapReached {
			t.Fatal("se esperaba CapReached")
		}
		if set.Duration() != 96*time.Hour || len(set.MarkedHours) != 4 {
			t.Fatalf("duración %v, horas %v", set.Duration(), set.MarkedHours)
		}
		for _, hr := range set.MarkedHours {
			if hr.Equal(h(9, 0)) {
				t.Fatal("la marca empatada más tardía no debía entrar")
			}
		}
		if ratio := ExcludedRatio(set, from, to); !approxEq(ratio, 0.4) {
			t.Fatalf("ratio = %v", ratio)
		}
	})

	t.Run("sin marcas ⇒ conjunto vacío", func(t *testing.T) {
		set := policy.Apply(nil, from, to)
		if len(set.Periods) != 0 || len(set.MarkedHours) != 0 || set.CapReached {
			t.Fatalf("set = %+v", set)
		}
	})
}
