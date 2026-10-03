package baseline

import "time"

// Estados de EffectiveBaseline.
const (
	StatusAvailable = "AVAILABLE"
	StatusNone      = "NONE"
)

// EffectiveBaseline es la línea base vigente de un punto en un instante.
// Solo tipos stdlib: es el contrato público que consume Processing.
type EffectiveBaseline struct {
	MeasuringPointID  int64
	Status            string // "AVAILABLE" | "NONE"
	VersionID         int64
	Version           int
	Confidence        string // "NORMAL" | "LOW"
	ValidFrom         time.Time
	DeviceFirstSeenAt *time.Time
	Signal            *SignalStats
	Components        []ComponentStats
	Flags             []string
	AlgorithmVersion  string
}

// SignalStats es la línea base de señal.
type SignalStats struct {
	MedianB2      float64
	ThetaD        float64
	P90Planitud   *float64
	MinElevatedB2 *float64 // nil ⇒ SIGNAL_DEGENERATE
	DeltaMin      float64
}

// ComponentStats es la estadística histórica de un componente.
type ComponentStats struct {
	WindowHours int     // 6 | 24
	Component   string  // "D" | "P" | "EX" | "C"
	Median, MAD float64 // MAD crudo
	NWindows    int
}
