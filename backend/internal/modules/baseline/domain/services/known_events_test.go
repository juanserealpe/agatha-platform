package services

import (
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

func TestExclusionPolicyWithKnownEvents(t *testing.T) {
	policy := NewExclusionPolicy(DefaultConfig())
	from := day0
	to := day0.AddDate(0, 0, 10) // 240 h ⇒ tope 96 h
	h := func(d, hour int) time.Time { return day0.AddDate(0, 0, d).Add(time.Duration(hour) * time.Hour) }
	known := []entities.KnownEvent{{From: h(5, 0), To: h(6, 0), Label: "fuga"}}

	t.Run("sin marcas, el evento conocido queda excluido", func(t *testing.T) {
		set := policy.ApplyWithKnown(nil, known, from, to)

		if len(set.Periods) != 1 || !set.Periods[0].From.Equal(h(5, 0)) || !set.Periods[0].To.Equal(h(6, 0)) || set.Periods[0].PeakI != 0 {
			t.Fatalf("periods = %+v", set.Periods)
		}
		if len(set.MarkedHours) != 0 || set.CapReached || set.Duration() != 24*time.Hour {
			t.Fatalf("set = %+v", set)
		}
		if !set.Excludes(h(5, 12)) || set.Excludes(h(6, 0)) {
			t.Fatal("el evento conocido se excluye en [From, To)")
		}
	})

	t.Run("se fusiona con una marca solapada y conserva el pico", func(t *testing.T) {
		set := policy.ApplyWithKnown([]HourMark{{T: h(5, 12), I: 0.5}}, known, from, to)

		if len(set.Periods) != 1 {
			t.Fatalf("periods = %+v", set.Periods)
		}
		p := set.Periods[0]
		if !p.From.Equal(h(4, 12)) || !p.To.Equal(h(6, 0)) || p.PeakI != 0.5 {
			t.Fatalf("period = %+v", p)
		}
		if len(set.MarkedHours) != 1 || set.Duration() != 36*time.Hour {
			t.Fatalf("set = %+v", set)
		}
	})

	t.Run("el evento conocido cuenta para el tope", func(t *testing.T) {
		// 24 h conocidas + 3 marcas disjuntas = 96 h (tope); la cuarta lo supera.
		marks := []HourMark{{T: h(1, 0), I: 0.9}, {T: h(2, 0), I: 0.8}, {T: h(3, 0), I: 0.7}, {T: h(8, 0), I: 0.6}}
		set := policy.ApplyWithKnown(marks, known, from, to)

		if !set.CapReached || len(set.MarkedHours) != 3 || set.Duration() != 96*time.Hour {
			t.Fatalf("set = %+v, duración = %v", set, set.Duration())
		}
	})

	t.Run("recorta el evento al histórico e ignora los que caen fuera", func(t *testing.T) {
		events := []entities.KnownEvent{
			{From: from.Add(-48 * time.Hour), To: h(0, 6)},
			{From: h(9, 12), To: to.Add(48 * time.Hour)},
			{From: to.Add(time.Hour), To: to.Add(5 * time.Hour)},
			{From: from.Add(-5 * time.Hour), To: from},
		}
		set := policy.ApplyWithKnown(nil, events, from, to)

		want := []entities.ExcludedPeriod{{From: from, To: h(0, 6)}, {From: h(9, 12), To: to}}
		if !reflect.DeepEqual(set.Periods, want) {
			t.Fatalf("periods = %+v, se esperaba %+v", set.Periods, want)
		}
	})

	t.Run("sin eventos conocidos equivale a Apply", func(t *testing.T) {
		marks := []HourMark{{T: h(3, 0), I: 0.3}, {T: h(3, 1), I: 0.5}}
		if got, want := policy.ApplyWithKnown(marks, nil, from, to), policy.Apply(marks, from, to); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
}

func TestEstimateExcludingKnownEvents(t *testing.T) {
	from, to := day0, day0.AddDate(0, 0, 30)
	ep := episode{from: day0.AddDate(0, 0, 15), to: day0.AddDate(0, 0, 15).Add(12 * time.Hour)}
	est := NewRobustBaselineEstimator(DefaultConfig())

	t.Run("el evento conocido se excluye desde la primera pasada", func(t *testing.T) {
		samples := synthetic(from, 30, ep)
		known := []entities.KnownEvent{{From: ep.from, To: ep.to, Label: "fuga"}}

		auto := est.Estimate(samples, from, to)
		res := est.EstimateExcluding(samples, from, to, known)

		if res.Status != EstimationEstimated {
			t.Fatalf("status = %s", res.Status)
		}
		if !entities.HasFlag(res.Flags, entities.FlagKnownEventsExcluded) {
			t.Fatalf("flags = %v, falta KNOWN_EVENTS_EXCLUDED", res.Flags)
		}
		if entities.HasFlag(auto.Flags, entities.FlagKnownEventsExcluded) {
			t.Fatal("sin eventos conocidos no debe marcar KNOWN_EVENTS_EXCLUDED")
		}
		// La pasada 1 ya se calcula sin el episodio: no arranca con θ_D inflado.
		if res.Passes[0].ExcludedRatio <= 0 || res.Passes[0].ThetaD > auto.Passes[0].ThetaD {
			t.Fatalf("pasada 1 = %+v (automática: %+v)", res.Passes[0], auto.Passes[0])
		}
		for ts := ep.from; ts.Before(ep.to); ts = ts.Add(time.Hour) {
			if !res.Exclusion.Excludes(ts) {
				t.Fatalf("%v no quedó excluido", ts)
			}
		}
	})

	t.Run("excluye un evento que el barrido no detecta", func(t *testing.T) {
		samples := synthetic(from, 30) // sin episodio: nada que marcar en esos días
		event := entities.KnownEvent{From: day0.AddDate(0, 0, 10), To: day0.AddDate(0, 0, 12), Label: "maniobra"}

		res := est.EstimateExcluding(samples, from, to, []entities.KnownEvent{event})

		if !res.Exclusion.Excludes(event.From) || !res.Exclusion.Excludes(event.To.Add(-time.Minute)) {
			t.Fatalf("el evento no quedó excluido: %+v", res.Exclusion.Periods)
		}
		// 2 días = 192 registros menos en la señal.
		if res.Signal.NRecords > 30*96-192 {
			t.Fatalf("n = %d, se esperaban como mucho %d", res.Signal.NRecords, 30*96-192)
		}
		if res.Summary.ExcludedRatio < 2.0/30 {
			t.Fatalf("ExcludedRatio = %v", res.Summary.ExcludedRatio)
		}
	})

	t.Run("un evento fuera del histórico no cambia el resultado", func(t *testing.T) {
		samples := synthetic(from, 30, ep)
		outside := []entities.KnownEvent{{From: from.AddDate(0, 0, -10), To: from.AddDate(0, 0, -5)}}

		if got, want := est.EstimateExcluding(samples, from, to, outside), est.Estimate(samples, from, to); !reflect.DeepEqual(got, want) {
			t.Fatalf("flags = %v, se esperaba el mismo resultado que sin eventos (%v)", got.Flags, want.Flags)
		}
	})
}
