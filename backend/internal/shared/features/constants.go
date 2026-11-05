package features

import "time"

// AlgorithmVersion identifica la versión de las fórmulas del kernel. Cualquier
// cambio de fórmula, piso o constante DEBE subir esta versión.
const AlgorithmVersion = "1.0.0"

const (
	// Slot es la cadencia nominal de registros del dispositivo.
	Slot = 15 * time.Minute
	// MaxGap es la separación máxima entre samples con B2 válido que no corta una racha.
	MaxGap = 20 * time.Minute
	// CoverageMinPct es la cobertura mínima (%) para que una ventana sea calculable.
	CoverageMinPct = 70.0
	// MinCoherenceSamples es el mínimo de samples con B1, B2 y Planitud para calcular C.
	MinCoherenceSamples = 40

	// PFloorHours es el piso del MAD de P (horas).
	PFloorHours = 0.25
	// CFloor es el piso del MAD de C.
	CFloor = 0.01

	// MADScale convierte el MAD crudo en un estimador de la desviación estándar.
	MADScale = 1.4826
	// SigmoidK y SigmoidZ0 parametrizan s(z) = 1 / (1 + e^(−k·(z − z0))).
	SigmoidK  = 0.6
	SigmoidZ0 = 2.0
)

// Nombres de componentes (coinciden con los valores persistidos).
const (
	ComponentD  = "D"
	ComponentP  = "P"
	ComponentEx = "EX"
	ComponentC  = "C"
)

// Nombres de series usados en Components.ConstantSeries.
const (
	SeriesB1       = "B1"
	SeriesB2       = "B2"
	SeriesPlanitud = "PLANITUD"
)

// ExpectedSamples devuelve N_esperados para una ventana de windowHours horas
// (24 para 6 h, 96 para 24 h).
func ExpectedSamples(windowHours int) int {
	return int(time.Duration(windowHours) * time.Hour / Slot)
}
