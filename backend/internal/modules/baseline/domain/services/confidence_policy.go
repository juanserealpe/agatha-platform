package services

import "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"

// ConfidencePolicy decide la confianza de una versión.
type ConfidencePolicy struct {
	normalWindows int
}

func NewConfidencePolicy(cfg Config) ConfidencePolicy {
	return ConfidencePolicy{normalWindows: cfg.NormalComponentWindows}
}

// Evaluate: NORMAL si están las 7 combinaciones (6 h: D, P, EX; 24 h: D, P, EX, C)
// y todas tienen NWindows ≥ 60. En cualquier otro caso LOW: una combinación
// faltante impide calcular I en tiempo real.
func (p ConfidencePolicy) Evaluate(stats []entities.ComponentStat) entities.Confidence {
	for _, spec := range HistoricalWindows {
		if !HasCompleteStats(stats, spec.Hours) {
			return entities.ConfidenceLow
		}
	}

	for _, st := range stats {
		if st.NWindows < p.normalWindows {
			return entities.ConfidenceLow
		}
	}

	return entities.ConfidenceNormal
}
