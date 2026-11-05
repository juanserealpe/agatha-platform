package entities

// EstimationSummary resume la estimación iterativa con exclusión de anómalos.
type EstimationSummary struct {
	Passes        int
	Converged     bool
	ExcludedRatio float64 // fracción [0, 1] del histórico excluida
	CapReached    bool
}

// PassReport describe una pasada de la estimación (diagnóstico, no se persiste).
type PassReport struct {
	Pass          int
	SignalOK      bool
	MedianB2      float64
	ThetaD        float64
	ExcludedRatio float64 // fracción con la que se calculó esta pasada
	Windows6h     int     // ventanas válidas de 6 h
	Windows24h    int     // ventanas válidas de 24 h
	MarkedHours   int     // horas marcadas aceptadas por esta pasada
	CapReached    bool
}
