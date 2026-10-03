package services

import (
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

func TestRobustBaselineEstimator(t *testing.T) {
	from, to := day0, day0.AddDate(0, 0, 30)
	ep := episode{from: day0.AddDate(0, 0, 15), to: day0.AddDate(0, 0, 15).Add(12 * time.Hour)}

	t.Run("converge y registra las pasadas", func(t *testing.T) {
		res := NewRobustBaselineEstimator(DefaultConfig()).Estimate(synthetic(from, 30, ep), from, to)

		if res.Status != EstimationEstimated || !res.Summary.Converged {
			t.Fatalf("status %s, summary %+v, flags %v", res.Status, res.Summary, res.Flags)
		}
		if res.Summary.Passes < 2 || len(res.Passes) != res.Summary.Passes {
			t.Fatalf("passes = %d, reports = %d", res.Summary.Passes, len(res.Passes))
		}
		if res.Summary.ExcludedRatio <= 0 || res.Summary.ExcludedRatio > 0.4 {
			t.Fatalf("ExcludedRatio = %v", res.Summary.ExcludedRatio)
		}
		if !res.Exclusion.OverlapsRange(ep.from, ep.to) {
			t.Fatalf("la exclusión no cubre el episodio: %+v", res.Exclusion.Periods)
		}
		if entities.HasFlag(res.Flags, entities.FlagBaselineReview) {
			t.Fatalf("flags inesperados: %v", res.Flags)
		}
		// El episodio infla θ_D en la pasada 1; al excluirlo baja.
		if res.Passes[0].ThetaD < res.Signal.ThetaD {
			t.Fatalf("θ_D pasada 1 = %v < final %v", res.Passes[0].ThetaD, res.Signal.ThetaD)
		}
	})

	t.Run("no converge en el máximo de pasadas ⇒ BASELINE_REVIEW", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxPasses = 1
		res := NewRobustBaselineEstimator(cfg).Estimate(synthetic(from, 30, ep), from, to)

		if res.Summary.Converged || res.Summary.Passes != 1 {
			t.Fatalf("summary = %+v", res.Summary)
		}
		if !entities.HasFlag(res.Flags, entities.FlagBaselineReview) {
			t.Fatalf("flags = %v", res.Flags)
		}
		// Se persiste la base calculada con E₀ = ∅.
		if len(res.Exclusion.Periods) != 0 || res.Summary.ExcludedRatio != 0 {
			t.Fatalf("exclusión = %+v", res.Exclusion)
		}
	})

	t.Run("pasada 1 sin estadísticas de 24 h ⇒ NO_EXCLUSION_HISTORY", func(t *testing.T) {
		short := day0.AddDate(0, 0, 3)
		res := NewRobustBaselineEstimator(DefaultConfig()).Estimate(synthetic(from, 3), from, short)

		if res.Status != EstimationEstimated || res.Summary.Passes != 1 || res.Summary.Converged {
			t.Fatalf("status %s, summary %+v", res.Status, res.Summary)
		}
		if !entities.HasFlag(res.Flags, entities.FlagNoExclusionHistory) || entities.HasFlag(res.Flags, entities.FlagBaselineReview) {
			t.Fatalf("flags = %v", res.Flags)
		}
		if res.Confidence != entities.ConfidenceLow {
			t.Fatalf("confianza = %s", res.Confidence)
		}
	})

	t.Run("menos de 96 registros ⇒ INSUFFICIENT_HISTORY", func(t *testing.T) {
		samples := synthetic(from, 1)[:95]
		res := NewRobustBaselineEstimator(DefaultConfig()).Estimate(samples, from, to)
		if res.Status != EstimationInsufficientHistory {
			t.Fatalf("status = %s", res.Status)
		}
	})

	t.Run("determinismo: misma entrada ⇒ mismo resultado", func(t *testing.T) {
		est := NewRobustBaselineEstimator(DefaultConfig())
		a := est.Estimate(synthetic(from, 30, ep), from, to)
		b := est.Estimate(synthetic(from, 30, ep), from, to)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("dos estimaciones con la misma entrada difieren")
		}
	})
}

func TestRecalculationPolicy(t *testing.T) {
	policy := NewRecalculationPolicy(DefaultConfig())
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	version := func(age time.Duration, conf entities.Confidence, flags ...entities.Flag) *entities.BaselineVersion {
		return &entities.BaselineVersion{CreatedAt: now.Add(-age), Confidence: conf, Flags: flags}
	}

	tests := []struct {
		name        string
		latest      *entities.BaselineVersion
		wantDue     bool
		wantTrigger entities.Trigger
	}{
		{"sin versión", nil, true, entities.TriggerBootstrap},
		{"LOW con 24 h exactas", version(24*time.Hour, entities.ConfidenceLow), true, entities.TriggerLowConfidence},
		{"LOW con 24 h − 1 s", version(24*time.Hour-time.Second, entities.ConfidenceLow), false, ""},
		{"NO_EXCLUSION_HISTORY con 24 h", version(24*time.Hour, entities.ConfidenceNormal, entities.FlagNoExclusionHistory), true, entities.TriggerLowConfidence},
		{"NORMAL con 168 h exactas", version(168*time.Hour, entities.ConfidenceNormal), true, entities.TriggerScheduled},
		{"NORMAL con 168 h − 1 s", version(168*time.Hour-time.Second, entities.ConfidenceNormal), false, ""},
		{"NORMAL con 24 h", version(24*time.Hour, entities.ConfidenceNormal), false, ""},
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

// Criterio de aceptación 10: un recálculo de 90 días termina en < 5 s.
func TestRobustBaselineEstimatorPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("rendimiento: se omite con -short")
	}
	from, to := day0, day0.AddDate(0, 0, 90)
	ep := episode{from: day0.AddDate(0, 0, 40), to: day0.AddDate(0, 0, 40).Add(18 * time.Hour)}
	samples := synthetic(from, 90, ep)

	start := time.Now()
	res := NewRobustBaselineEstimator(DefaultConfig()).Estimate(samples, from, to)
	elapsed := time.Since(start)

	t.Logf("90 días, %d samples: %d pasadas en %v", len(samples), res.Summary.Passes, elapsed)
	if elapsed > 5*time.Second {
		t.Fatalf("el recálculo tardó %v (> 5 s)", elapsed)
	}
}
