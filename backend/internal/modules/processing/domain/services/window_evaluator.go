package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// EvaluatorConfig son los parámetros del evaluador que no vienen del kernel.
type EvaluatorConfig struct {
	// ConfigVersion identifica lo que fuera del kernel cambia un resultado
	// (IDs de métricas), p. ej. "m=12,13,18".
	ConfigVersion string
	// SilentPeriod es el periodo inicial del dispositivo marcado como silencioso (7 d).
	SilentPeriod time.Duration
}

// EvaluationInput es todo lo que necesita la evaluación de una hora.
type EvaluationInput struct {
	PointID int64
	At      time.Time // hora evaluada t (en punto, UTC)
	// Window24 y Window6 son los samples normalizados de [t − 24 h, t) y [t − 6 h, t).
	Window24, Window6 []features.Sample
	Baseline          entities.BaselineSnapshot
	Trigger           entities.Trigger
	BatchSeq          *int64
	CreatedAt         time.Time
}

// WindowEvaluator evalúa las ventanas de 24 h y 6 h de una hora con el kernel
// compartido y aplica la precedencia de estados. No reimplementa fórmulas.
type WindowEvaluator struct {
	cfg EvaluatorConfig
}

func NewWindowEvaluator(cfg EvaluatorConfig) WindowEvaluator {
	return WindowEvaluator{cfg: cfg}
}

// Evaluate devuelve las evaluaciones de 24 h y 6 h de la hora. Primero 24 h:
// la de 6 h usa la C de 24 h y la estadística de C de 24 h.
func (e WindowEvaluator) Evaluate(in EvaluationInput) (w24, w6 entities.FeatureEvaluation) {
	sig := in.Baseline.Signal

	comps24 := features.ComputeComponents(in.Window24, entities.Window24h, sig, true)

	comps6 := features.ComputeComponents(in.Window6, entities.Window6h, sig, false)
	comps6.C, comps6.HasC = comps24.C, comps24.HasC
	comps6.ConstantSeries = comps24.ConstantSeries

	stats24 := in.Baseline.Stats[entities.Window24h]
	stats6 := in.Baseline.Stats[entities.Window6h]
	stats6.C, stats6.HasC = stats24.C, stats24.HasC

	w24 = e.evaluate(in, entities.Window24h, in.Window24, comps24, stats24, false)
	w6 = e.evaluate(in, entities.Window6h, in.Window6, comps6, stats6, true)

	return w24, w6
}

// evaluate aplica la precedencia de la sección 8: el primer motivo que se
// cumple decide el estado y qué se guarda.
func (e WindowEvaluator) evaluate(
	in EvaluationInput,
	windowHours int,
	window []features.Sample,
	comps features.Components,
	stats features.ComponentStats,
	cFrom24h bool,
) entities.FeatureEvaluation {
	ev := e.base(in, windowHours, window, comps, cFrom24h)
	snap := in.Baseline

	switch {
	case !snap.Available:
		return noCalculable(ev, entities.ReasonNoSignalBaseline)
	case snap.IsSignalDegenerate():
		return noCalculable(ev, entities.ReasonSignalDegenerate)
	case !comps.Calculable:
		return noCalculable(ev, entities.ReasonLowCoverage)
	}

	ev.D, ev.PHours, ev.Ex = ptr(comps.D), ptr(comps.P), ptr(comps.Ex)
	if comps.HasC {
		ev.C = ptr(comps.C)
	}
	if comps.HasDPl {
		ev.DPl = ptr(comps.DPl)
	}
	for _, series := range comps.ConstantSeries {
		ev.Flags = append(ev.Flags, entities.FlagConstantSeriesPrefix+series)
	}

	if !hasAllStats(stats) {
		return noCalculable(ev, entities.ReasonNoComponentBaseline)
	}
	if !comps.HasC {
		return noCalculable(ev, entities.ReasonInsufficientCoherenceData)
	}

	scores, ok := features.Score(comps, stats, windowHours, snap.Signal)
	if !ok {
		// Inalcanzable: Score solo falla por C o estadísticas, ya verificados.
		return noCalculable(ev, entities.ReasonNoComponentBaseline)
	}

	ev.ZD, ev.ZP, ev.ZEx, ev.ZC = ptr(scores.ZD), ptr(scores.ZP), ptr(scores.ZEx), ptr(scores.ZC)
	ev.SD, ev.SP, ev.SEx, ev.SC = ptr(scores.SD), ptr(scores.SP), ptr(scores.SEx), ptr(scores.SC)
	ev.I = ptr(scores.I)
	for _, component := range scores.FloorsApplied {
		ev.Flags = append(ev.Flags, entities.FlagMADFloorAppliedPrefix+component)
	}

	ev.Status = entities.StatusCalculated
	return ev
}

// base arma los campos que toda fila lleva, cualquiera sea su estado.
func (e WindowEvaluator) base(
	in EvaluationInput,
	windowHours int,
	window []features.Sample,
	comps features.Components,
	cFrom24h bool,
) entities.FeatureEvaluation {
	snap := in.Baseline
	at := in.At.UTC()

	ev := entities.FeatureEvaluation{
		MeasuringPointID: in.PointID,
		WindowHours:      windowHours,
		EvaluatedAt:      at,
		WindowStart:      at.Add(-time.Duration(windowHours) * time.Hour),
		NExpected:        comps.NExpected,
		NReceived:        comps.NReceived,
		NValid:           comps.NValid,
		Coverage:         comps.Coverage,
		CFrom24h:         cFrom24h,
		Silent:           e.isSilent(snap.FirstSeenAt, at),
		Flags:            []string{},
		AlgorithmVersion: features.AlgorithmVersion,
		ConfigVersion:    e.cfg.ConfigVersion,
		Trigger:          in.Trigger,
		SourceBatchSeq:   in.BatchSeq,
		CreatedAt:        in.CreatedAt.UTC(),
	}

	if len(window) > 0 {
		ev.InputMaxTs = ptr(window[len(window)-1].T.UTC())
	}

	if snap.Available {
		ev.LowConfidence = snap.Confidence == entities.ConfidenceLow
		ev.BaselineVersionID = ptr(snap.VersionID)
		ev.BaselineVersion = ptr(snap.Version)
		ev.BaselineValidFrom = ptr(snap.ValidFrom.UTC())
	}

	return ev
}

// isSilent: t < first_seen + SilentPeriod. Un dispositivo sin ningún registro
// todavía está en su periodo inicial.
func (e WindowEvaluator) isSilent(firstSeen *time.Time, at time.Time) bool {
	if firstSeen == nil {
		return true
	}
	return at.Before(firstSeen.Add(e.cfg.SilentPeriod))
}

func noCalculable(ev entities.FeatureEvaluation, reason entities.NoCalcReason) entities.FeatureEvaluation {
	ev.Status = entities.StatusNoCalculable
	ev.NoCalcReason = reason
	return ev
}

// hasAllStats: D, P, Ex de la ventana y C (de 24 h) con al menos una ventana histórica.
func hasAllStats(st features.ComponentStats) bool {
	return st.D.N > 0 && st.P.N > 0 && st.Ex.N > 0 && st.HasC && st.C.N > 0
}

func ptr[T any](v T) *T { return &v }
