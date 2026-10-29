package services

import (
	"sort"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// defaultDeltaMin se usa cuando B2 tiene un solo valor distinto (no existe
// diferencia positiva). Es la resolución práctica de B2 según el analista.
const defaultDeltaMin = 1.0

// SignalCalculation es el resultado del cálculo de la línea base de señal.
type SignalCalculation struct {
	Signal entities.SignalBaseline
	Flags  []entities.Flag
	OK     bool // false ⇒ menos de MinSignalRecords registros con B2 válido
}

// SignalBaselineCalculator calcula mediana_B2, θ_D, P90_Planitud y los
// insumos de pisos sobre los registros elegibles (paso 2 de la sección 9.2).
type SignalBaselineCalculator struct {
	minRecords int
}

func NewSignalBaselineCalculator(cfg Config) SignalBaselineCalculator {
	return SignalBaselineCalculator{minRecords: cfg.MinSignalRecords}
}

// Calculate recibe los samples elegibles (ya sin los excluidos).
func (c SignalBaselineCalculator) Calculate(eligible []features.Sample) SignalCalculation {
	var b2, pl []float64
	for _, s := range eligible {
		if s.HasB2 {
			b2 = append(b2, s.B2)
		}
		if s.HasPl {
			pl = append(pl, s.Pl)
		}
	}

	if len(b2) < c.minRecords {
		return SignalCalculation{Flags: []entities.Flag{}}
	}

	sig := entities.SignalBaseline{
		MedianB2:         features.Median(b2),
		ThetaD:           features.Quantile(b2, 0.9),
		DeltaMin:         deltaMin(b2),
		NRecords:         len(b2),
		NPlanitudRecords: len(pl),
	}

	if len(pl) > 0 {
		p90 := features.Quantile(pl, 0.9)
		sig.P90Planitud = &p90
	}

	flags := []entities.Flag{}

	if minElevated, ok := minAbove(b2, sig.ThetaD); ok {
		sig.MinElevatedB2 = &minElevated
	} else {
		flags = append(flags, entities.FlagSignalDegenerate)
	}

	if sig.ThetaD == sig.MedianB2 {
		flags = append(flags, entities.FlagSignalLowVariability)
	}

	return SignalCalculation{Signal: sig, Flags: flags, OK: true}
}

// minAbove devuelve min{x > threshold}.
func minAbove(xs []float64, threshold float64) (float64, bool) {
	var (
		best  float64
		found bool
	)
	for _, x := range xs {
		if x > threshold && (!found || x < best) {
			best, found = x, true
		}
	}
	return best, found
}

// deltaMin devuelve la menor diferencia positiva entre valores distintos.
func deltaMin(xs []float64) float64 {
	s := make([]float64, len(xs))
	copy(s, xs)
	sort.Float64s(s)

	var (
		best  float64
		found bool
	)
	for i := 1; i < len(s); i++ {
		if d := s[i] - s[i-1]; d > 0 && (!found || d < best) {
			best, found = d, true
		}
	}

	if !found {
		return defaultDeltaMin
	}
	return best
}
