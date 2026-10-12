package entities

// Confidence es el nivel de confianza de una versión de línea base.
type Confidence string

const (
	ConfidenceNormal Confidence = "NORMAL"
	ConfidenceLow    Confidence = "LOW"
)

// Flag es una marca de calidad de una versión de línea base.
type Flag string

const (
	FlagSignalDegenerate     Flag = "SIGNAL_DEGENERATE"
	FlagSignalLowVariability Flag = "SIGNAL_LOW_VARIABILITY"
	FlagNoExclusionHistory   Flag = "NO_EXCLUSION_HISTORY"
	FlagExclusionCapReached  Flag = "EXCLUSION_CAP_REACHED"
	FlagBaselineReview       Flag = "BASELINE_REVIEW"
	// FlagKnownEventsExcluded: el histórico incluía eventos conocidos que se
	// excluyeron del cálculo (ver KnownEvent).
	FlagKnownEventsExcluded Flag = "KNOWN_EVENTS_EXCLUDED"
)

// flagOrder fija el orden canónico de los flags (determinismo al persistir).
var flagOrder = []Flag{
	FlagSignalDegenerate,
	FlagSignalLowVariability,
	FlagNoExclusionHistory,
	FlagExclusionCapReached,
	FlagBaselineReview,
	FlagKnownEventsExcluded,
}

// NormalizeFlags elimina duplicados y ordena los flags en el orden canónico.
func NormalizeFlags(flags []Flag) []Flag {
	present := make(map[Flag]bool, len(flags))
	for _, f := range flags {
		present[f] = true
	}

	out := make([]Flag, 0, len(present))
	for _, f := range flagOrder {
		if present[f] {
			out = append(out, f)
		}
	}

	return out
}

// HasFlag indica si flags contiene f.
func HasFlag(flags []Flag, f Flag) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}

// Trigger es el motivo por el que se creó una versión.
type Trigger string

const (
	TriggerBootstrap     Trigger = "BOOTSTRAP"
	TriggerScheduled     Trigger = "SCHEDULED"
	TriggerLowConfidence Trigger = "LOW_CONFIDENCE"
	TriggerManual        Trigger = "MANUAL"
)
