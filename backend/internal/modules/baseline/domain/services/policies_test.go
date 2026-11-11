package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

func TestDefaultConfig_MatchesTheSpec(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.HistoryDays != 90 || cfg.MinSignalRecords != 96 || cfg.MinComponentWindows != 20 ||
		cfg.NormalComponentWindows != 60 || cfg.ExclusionICutoff != 0.18 || cfg.ExclusionCapRatio != 0.40 ||
		cfg.MaxPasses != 5 || cfg.RecalcInterval != 168*time.Hour || cfg.LowRecalcInterval != 24*time.Hour ||
		cfg.KnownEventMargin != 2*time.Hour {
		t.Fatalf("configuración por defecto distinta a la especificación: %+v", cfg)
	}
	if cfg.HistoryWindow() != 90*24*time.Hour {
		t.Fatalf("HistoryWindow = %v, se esperaba 2160h", cfg.HistoryWindow())
	}
}

func TestConfig_KnownEventsRangeAddsTheMarginOnBothEnds(t *testing.T) {
	from, to := DefaultConfig().KnownEventsRange(at(10), at(20))

	if !from.Equal(at(8)) || !to.Equal(at(22)) {
		t.Fatalf("rango = [%v, %v), se esperaba [%v, %v)", from, to, at(8), at(22))
	}
}

func TestRecalculationPolicy_IsDue(t *testing.T) {
	policy := NewRecalculationPolicy(DefaultConfig())
	now := at(1000)
	version := func(ageHours int, confidence entities.Confidence, flags ...entities.Flag) *entities.BaselineVersion {
		return &entities.BaselineVersion{CreatedAt: now.Add(-time.Duration(ageHours) * time.Hour), Confidence: confidence, Flags: flags}
	}

	tests := []struct {
		name        string
		latest      *entities.BaselineVersion
		wantDue     bool
		wantTrigger entities.Trigger
	}{
		{name: "sin versión arranca con bootstrap", latest: nil, wantDue: true, wantTrigger: entities.TriggerBootstrap},
		{name: "normal recién creada", latest: version(10, entities.ConfidenceNormal), wantDue: false},
		{name: "normal de 167 h aún no vence", latest: version(167, entities.ConfidenceNormal), wantDue: false},
		{name: "normal de 168 h vence por calendario", latest: version(168, entities.ConfidenceNormal), wantDue: true, wantTrigger: entities.TriggerScheduled},
		{name: "baja de 23 h aún no vence", latest: version(23, entities.ConfidenceLow), wantDue: false},
		{name: "baja de 24 h vence por confianza baja", latest: version(24, entities.ConfidenceLow), wantDue: true, wantTrigger: entities.TriggerLowConfidence},
		{
			name:    "normal sin historial de exclusión se trata como débil",
			latest:  version(24, entities.ConfidenceNormal, entities.FlagNoExclusionHistory),
			wantDue: true, wantTrigger: entities.TriggerLowConfidence,
		},
		{name: "baja de 200 h sigue siendo confianza baja", latest: version(200, entities.ConfidenceLow), wantDue: true, wantTrigger: entities.TriggerLowConfidence},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			due, trigger := policy.IsDue(tt.latest, now)
			if due != tt.wantDue || trigger != tt.wantTrigger {
				t.Fatalf("IsDue = (%v, %q), se esperaba (%v, %q)", due, trigger, tt.wantDue, tt.wantTrigger)
			}
		})
	}
}

func TestConfidencePolicy_Evaluate(t *testing.T) {
	policy := NewConfidencePolicy(DefaultConfig())

	if got := policy.Evaluate(completeStats(60)); got != entities.ConfidenceNormal {
		t.Fatalf("stats completas con 60 ventanas = %s, se esperaba NORMAL", got)
	}
	if got := policy.Evaluate(completeStats(59)); got != entities.ConfidenceLow {
		t.Fatalf("stats con 59 ventanas = %s, se esperaba LOW", got)
	}

	withoutC := []entities.ComponentStat{}
	for _, st := range completeStats(100) {
		if !(st.WindowHours == 24 && st.Component == features.ComponentC) {
			withoutC = append(withoutC, st)
		}
	}
	if got := policy.Evaluate(withoutC); got != entities.ConfidenceLow {
		t.Fatalf("sin C de 24 h = %s, se esperaba LOW", got)
	}
	if got := policy.Evaluate(nil); got != entities.ConfidenceLow {
		t.Fatalf("sin stats = %s, se esperaba LOW", got)
	}
}

func TestHasCompleteStats(t *testing.T) {
	stats := completeStats(30)
	if !HasCompleteStats(stats, 6) || !HasCompleteStats(stats, 24) {
		t.Fatal("stats completas reportadas como incompletas")
	}

	only6 := []entities.ComponentStat{}
	for _, st := range stats {
		if st.WindowHours == 6 {
			only6 = append(only6, st)
		}
	}
	if !HasCompleteStats(only6, 6) || HasCompleteStats(only6, 24) {
		t.Fatal("solo hay stats de 6 h: 24 h debe estar incompleta")
	}
}

func TestHistoricalWindowGenerator_Ends(t *testing.T) {
	gen := HistoricalWindowGenerator{}

	// from 00:30 + 6 h = 06:30 → primer fin alineado al paso de 3 h: 09:00; to incluido.
	got := gen.Ends(at(0).Add(30*time.Minute), at(15), WindowSpec{Hours: 6, Step: 3 * time.Hour})
	want := []time.Time{at(9), at(12), at(15)}
	if len(got) != len(want) {
		t.Fatalf("fines = %v, se esperaba %v", got, want)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Fatalf("fines = %v, se esperaba %v", got, want)
		}
	}

	// from alineado: el primer fin es exactamente from + 24 h.
	aligned := gen.Ends(at(0), at(30), WindowSpec{Hours: 24, Step: 6 * time.Hour})
	if len(aligned) != 2 || !aligned[0].Equal(at(24)) || !aligned[1].Equal(at(30)) {
		t.Fatalf("fines alineados = %v, se esperaba [%v %v]", aligned, at(24), at(30))
	}

	if short := gen.Ends(at(0), at(5), WindowSpec{Hours: 6, Step: 3 * time.Hour}); len(short) != 0 {
		t.Fatalf("un rango menor que la ventana no tiene fines: %v", short)
	}
}

func TestSamplesIn_IsHalfOpen(t *testing.T) {
	samples := normalSamples(at(0), at(2)) // 8 muestras: 00:00 … 01:45

	got := samplesIn(samples, at(0).Add(15*time.Minute), at(1))
	if len(got) != 3 || !got[0].T.Equal(at(0).Add(15*time.Minute)) || !got[2].T.Equal(at(0).Add(45*time.Minute)) {
		t.Fatalf("samplesIn devolvió %d muestras desde %v", len(got), got[0].T)
	}
}
