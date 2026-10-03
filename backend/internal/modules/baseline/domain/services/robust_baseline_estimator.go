package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// EstimationStatus es el resultado global de la estimación.
type EstimationStatus string

const (
	EstimationEstimated           EstimationStatus = "ESTIMATED"
	EstimationInsufficientHistory EstimationStatus = "INSUFFICIENT_HISTORY"
)

// Estimation es la línea base estimada lista para versionar.
type Estimation struct {
	Status     EstimationStatus
	Signal     entities.SignalBaseline
	Components []entities.ComponentStat
	Confidence entities.Confidence
	Exclusion  entities.ExclusionSet // conjunto CON EL QUE se calculó la línea base
	Summary    entities.EstimationSummary
	Flags      []entities.Flag
	Passes     []entities.PassReport // diagnóstico por pasada (no se persiste)
}

// passBase es una línea base calculada con un conjunto excluido dado.
type passBase struct {
	signal     SignalCalculation
	components ComponentCalculation
	exclusion  entities.ExclusionSet
}

// RobustBaselineEstimator ejecuta la estimación iterativa con exclusión de
// anómalos (sección 9.1). Es puro: mismos samples y config ⇒ mismo resultado.
type RobustBaselineEstimator struct {
	maxPasses  int
	signal     SignalBaselineCalculator
	components ComponentBaselineCalculator
	scanner    HourlyAnomalyScanner
	exclusion  ExclusionPolicy
	confidence ConfidencePolicy
}

func NewRobustBaselineEstimator(cfg Config) RobustBaselineEstimator {
	maxPasses := cfg.MaxPasses
	if maxPasses < 1 {
		maxPasses = 1
	}

	return RobustBaselineEstimator{
		maxPasses:  maxPasses,
		signal:     NewSignalBaselineCalculator(cfg),
		components: NewComponentBaselineCalculator(cfg),
		scanner:    NewHourlyAnomalyScanner(cfg),
		exclusion:  NewExclusionPolicy(cfg),
		confidence: NewConfidencePolicy(cfg),
	}
}

// Estimate calcula la línea base sobre [from, to). samples debe estar normalizado.
func (e RobustBaselineEstimator) Estimate(samples []features.Sample, from, to time.Time) Estimation {
	var (
		reports  []entities.PassReport
		previous *passBase
		excluded = entities.ExclusionSet{Periods: []entities.ExcludedPeriod{}, MarkedHours: []time.Time{}}
	)

	for k := 1; k <= e.maxPasses; k++ {
		base := e.calculate(samples, excluded, from, to)
		report := e.report(k, base, from, to)

		if !base.signal.OK {
			reports = append(reports, report)
			if previous == nil {
				return Estimation{Status: EstimationInsufficientHistory, Passes: reports, Flags: []entities.Flag{}}
			}
			return e.result(*previous, k-1, false, reports, from, to, entities.FlagBaselineReview)
		}

		if !HasCompleteStats(base.components.Stats, scanWindowHours) ||
			entities.HasFlag(base.signal.Flags, entities.FlagSignalDegenerate) {
			reports = append(reports, report)
			extra := []entities.Flag{entities.FlagNoExclusionHistory}
			if k > 1 {
				extra = append(extra, entities.FlagBaselineReview)
			}
			return e.result(base, k, false, reports, from, to, extra...)
		}

		sig := base.signal.Signal.ToFeatureSignal()
		stats24 := entities.FeatureStats(base.components.Stats, scanWindowHours)
		next := e.exclusion.Apply(e.scanner.Scan(samples, sig, stats24, from, to), from, to)

		report.MarkedHours = len(next.MarkedHours)
		report.CapReached = next.CapReached
		reports = append(reports, report)

		if next.SameMarks(excluded) {
			return e.result(base, k, true, reports, from, to)
		}

		previous = &base
		excluded = next
	}

	// No convergió: se persiste la última base (calculada con E_{max−1}).
	return e.result(*previous, e.maxPasses, false, reports, from, to, entities.FlagBaselineReview)
}

// calculate es la sección 9.2: señal y componentes con un conjunto excluido.
func (e RobustBaselineEstimator) calculate(
	samples []features.Sample,
	excluded entities.ExclusionSet,
	from, to time.Time,
) passBase {
	eligible := make([]features.Sample, 0, len(samples))
	for _, s := range samples {
		if !excluded.Excludes(s.T) {
			eligible = append(eligible, s)
		}
	}

	base := passBase{signal: e.signal.Calculate(eligible), exclusion: excluded}
	if !base.signal.OK {
		return base
	}

	base.components = e.components.Calculate(samples, base.signal.Signal.ToFeatureSignal(), excluded, from, to)
	return base
}

func (e RobustBaselineEstimator) report(k int, base passBase, from, to time.Time) entities.PassReport {
	return entities.PassReport{
		Pass:          k,
		SignalOK:      base.signal.OK,
		MedianB2:      base.signal.Signal.MedianB2,
		ThetaD:        base.signal.Signal.ThetaD,
		ExcludedRatio: ExcludedRatio(base.exclusion, from, to),
		Windows6h:     base.components.Windows6h,
		Windows24h:    base.components.Windows24h,
	}
}

func (e RobustBaselineEstimator) result(
	base passBase,
	passes int,
	converged bool,
	reports []entities.PassReport,
	from, to time.Time,
	extra ...entities.Flag,
) Estimation {
	flags := append([]entities.Flag{}, base.signal.Flags...)
	flags = append(flags, extra...)
	if base.exclusion.CapReached {
		flags = append(flags, entities.FlagExclusionCapReached, entities.FlagBaselineReview)
	}

	return Estimation{
		Status:     EstimationEstimated,
		Signal:     base.signal.Signal,
		Components: base.components.Stats,
		Confidence: e.confidence.Evaluate(base.components.Stats),
		Exclusion:  base.exclusion,
		Summary: entities.EstimationSummary{
			Passes:        passes,
			Converged:     converged,
			ExcludedRatio: ExcludedRatio(base.exclusion, from, to),
			CapReached:    base.exclusion.CapReached,
		},
		Flags:  entities.NormalizeFlags(flags),
		Passes: reports,
	}
}
