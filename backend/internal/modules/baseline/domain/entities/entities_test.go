package entities

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

var t0 = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func h(n int) time.Time { return t0.Add(time.Duration(n) * time.Hour) }

func TestNormalizeFlags_DeduplicatesAndUsesTheCanonicalOrder(t *testing.T) {
	got := NormalizeFlags([]Flag{
		FlagKnownEventsExcluded, FlagBaselineReview, FlagSignalDegenerate, FlagBaselineReview, Flag("DESCONOCIDA"),
	})

	want := []Flag{FlagSignalDegenerate, FlagBaselineReview, FlagKnownEventsExcluded}
	if len(got) != len(want) {
		t.Fatalf("flags = %v, se esperaba %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("flags = %v, se esperaba %v", got, want)
		}
	}

	if empty := NormalizeFlags(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("sin flags se espera una lista vacía no nil, se obtuvo %v", empty)
	}
}

func TestHasFlag(t *testing.T) {
	flags := []Flag{FlagNoExclusionHistory}
	if !HasFlag(flags, FlagNoExclusionHistory) || HasFlag(flags, FlagBaselineReview) || HasFlag(nil, FlagBaselineReview) {
		t.Fatal("HasFlag no refleja la presencia del flag")
	}
}

func TestExcludedPeriod_IsHalfOpen(t *testing.T) {
	p := ExcludedPeriod{From: h(2), To: h(5)}

	tests := []struct {
		at   time.Time
		want bool
	}{
		{h(1), false},
		{h(2), true}, // From incluido
		{h(4), true},
		{h(5), false}, // To excluido
	}
	for _, tt := range tests {
		if got := p.Contains(tt.at); got != tt.want {
			t.Errorf("Contains(%v) = %v, se esperaba %v", tt.at, got, tt.want)
		}
	}
}

func TestExcludedPeriod_Overlaps(t *testing.T) {
	p := ExcludedPeriod{From: h(2), To: h(5)}

	tests := []struct {
		name     string
		from, to time.Time
		want     bool
	}{
		{"antes, tocando el borde", h(0), h(2), false},
		{"después, tocando el borde", h(5), h(8), false},
		{"cruza el inicio", h(1), h(3), true},
		{"adentro", h(3), h(4), true},
		{"lo contiene", h(0), h(9), true},
	}
	for _, tt := range tests {
		if got := p.Overlaps(tt.from, tt.to); got != tt.want {
			t.Errorf("%s: Overlaps = %v, se esperaba %v", tt.name, got, tt.want)
		}
	}
}

func TestExclusionSet(t *testing.T) {
	set := ExclusionSet{
		Periods:     []ExcludedPeriod{{From: h(0), To: h(24)}, {From: h(48), To: h(60)}},
		MarkedHours: []time.Time{h(24), h(60)},
	}

	if !set.Excludes(h(10)) || set.Excludes(h(30)) || !set.Excludes(h(59)) {
		t.Fatal("Excludes no respeta los periodos")
	}
	if !set.OverlapsRange(h(20), h(30)) || set.OverlapsRange(h(24), h(48)) {
		t.Fatal("OverlapsRange no respeta los periodos")
	}
	if got := set.Duration(); got != 36*time.Hour {
		t.Fatalf("Duration = %v, se esperaba 36h", got)
	}

	if !set.SameMarks(ExclusionSet{MarkedHours: []time.Time{h(24).In(time.FixedZone("COT", -5*3600)), h(60)}}) {
		t.Fatal("las mismas horas en otra zona horaria son las mismas marcas")
	}
	if set.SameMarks(ExclusionSet{MarkedHours: []time.Time{h(24)}}) {
		t.Fatal("distinta cantidad de marcas no son las mismas")
	}
	if set.SameMarks(ExclusionSet{MarkedHours: []time.Time{h(24), h(61)}}) {
		t.Fatal("marcas distintas no son las mismas")
	}
	if (ExclusionSet{}).Duration() != 0 || (ExclusionSet{}).Excludes(h(0)) {
		t.Fatal("un conjunto vacío no excluye nada")
	}
}

func TestFeatureStats_PicksTheWindowAndMarksC(t *testing.T) {
	stats := []ComponentStat{
		{WindowHours: 6, Component: features.ComponentD, Median: 1, MAD: 0.1, NWindows: 30},
		{WindowHours: 24, Component: features.ComponentD, Median: 2, MAD: 0.2, NWindows: 40},
		{WindowHours: 24, Component: features.ComponentP, Median: 3, MAD: 0.3, NWindows: 41},
		{WindowHours: 24, Component: features.ComponentEx, Median: 4, MAD: 0.4, NWindows: 42},
		{WindowHours: 24, Component: features.ComponentC, Median: 5, MAD: 0.5, NWindows: 43},
	}

	got24 := FeatureStats(stats, 24)
	if got24.D.Median != 2 || got24.P.N != 41 || got24.Ex.MAD != 0.4 || !got24.HasC || got24.C.Median != 5 {
		t.Fatalf("stats 24 h mal armadas: %+v", got24)
	}

	got6 := FeatureStats(stats, 6)
	if got6.D.Median != 1 || got6.HasC || got6.P.N != 0 {
		t.Fatalf("stats 6 h mal armadas: %+v", got6)
	}
}

func TestSignalBaseline_ToFeatureSignal(t *testing.T) {
	p90, minElevated := 7.5, 81.0
	full := SignalBaseline{MedianB2: 50, ThetaD: 80, DeltaMin: 0.5, P90Planitud: &p90, MinElevatedB2: &minElevated}.ToFeatureSignal()

	if full.MedianB2 != 50 || full.ThetaD != 80 || full.DeltaMin != 0.5 ||
		!full.HasP90Planitud || full.P90Planitud != 7.5 || !full.HasMinElevated || full.MinElevatedB2 != 81 {
		t.Fatalf("señal completa mal convertida: %+v", full)
	}

	bare := SignalBaseline{MedianB2: 50, ThetaD: 50}.ToFeatureSignal()
	if bare.HasP90Planitud || bare.HasMinElevated {
		t.Fatalf("los punteros nil deben quedar como ausentes: %+v", bare)
	}
}
