package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// RecalculationPolicy decide si a un punto le toca recalcular.
type RecalculationPolicy struct {
	interval    time.Duration
	lowInterval time.Duration
}

func NewRecalculationPolicy(cfg Config) RecalculationPolicy {
	return RecalculationPolicy{interval: cfg.RecalcInterval, lowInterval: cfg.LowRecalcInterval}
}

// IsDue aplica la tabla de la sección 8.2 sobre la última versión del punto.
func (p RecalculationPolicy) IsDue(latest *entities.BaselineVersion, now time.Time) (bool, entities.Trigger) {
	if latest == nil {
		return true, entities.TriggerBootstrap
	}

	age := now.Sub(latest.CreatedAt)

	weak := latest.Confidence == entities.ConfidenceLow ||
		entities.HasFlag(latest.Flags, entities.FlagNoExclusionHistory)
	if weak && age >= p.lowInterval {
		return true, entities.TriggerLowConfidence
	}

	if age >= p.interval {
		return true, entities.TriggerScheduled
	}

	return false, ""
}
