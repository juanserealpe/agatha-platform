package entities

// EvaluationStatus es el resultado de evaluar una ventana.
type EvaluationStatus string

const (
	StatusCalculated   EvaluationStatus = "CALCULATED"
	StatusNoCalculable EvaluationStatus = "NO_CALCULABLE"
)

// NoCalcReason explica por qué una ventana es NO_CALCULABLE. No es un error.
// El orden de las constantes es la precedencia (se aplica la primera que se cumple).
type NoCalcReason string

const (
	ReasonNone                      NoCalcReason = ""
	ReasonNoSignalBaseline          NoCalcReason = "NO_SIGNAL_BASELINE"
	ReasonSignalDegenerate          NoCalcReason = "SIGNAL_DEGENERATE"
	ReasonLowCoverage               NoCalcReason = "LOW_COVERAGE"
	ReasonNoComponentBaseline       NoCalcReason = "NO_COMPONENT_BASELINE"
	ReasonInsufficientCoherenceData NoCalcReason = "INSUFFICIENT_COHERENCE_DATA"
)

// Trigger es quién originó la evaluación.
type Trigger string

const (
	TriggerEvent      Trigger = "EVENT"
	TriggerReconciler Trigger = "RECONCILER"
	// TriggerBackfill es la carga del histórico (cmd/processing -backfill).
	TriggerBackfill Trigger = "BACKFILL"
	// TriggerManual es una evaluación a demanda (cmd/processing).
	TriggerManual Trigger = "MANUAL"
	// TriggerReplay solo lo usa cmd/replay; nunca se persiste.
	TriggerReplay Trigger = "REPLAY"
)

// Prefijos de las marcas de una evaluación (no cambian el estado).
const (
	FlagMADFloorAppliedPrefix = "MAD_FLOOR_APPLIED_"
	FlagConstantSeriesPrefix  = "CONSTANT_SERIES_"
)

// Longitudes de ventana evaluadas cada hora.
const (
	Window24h = 24
	Window6h  = 6
)
