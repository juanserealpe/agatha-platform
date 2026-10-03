package entities

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/features"

// SignalBaseline es la línea base de señal (B2 y Planitud) de una versión.
type SignalBaseline struct {
	MedianB2         float64
	ThetaD           float64  // P90 de B2
	P90Planitud      *float64 // nil si no hay Planitud
	MinElevatedB2    *float64 // nil ⇒ SIGNAL_DEGENERATE
	DeltaMin         float64  // menor diferencia positiva entre valores distintos de B2
	NRecords         int      // registros con B2 válido usados
	NPlanitudRecords int      // registros con Planitud válida usados
}

// ToFeatureSignal convierte la línea base de señal al tipo del kernel.
func (s SignalBaseline) ToFeatureSignal() features.Signal {
	sig := features.Signal{
		MedianB2: s.MedianB2,
		ThetaD:   s.ThetaD,
		DeltaMin: s.DeltaMin,
	}

	if s.P90Planitud != nil {
		sig.P90Planitud, sig.HasP90Planitud = *s.P90Planitud, true
	}

	if s.MinElevatedB2 != nil {
		sig.MinElevatedB2, sig.HasMinElevated = *s.MinElevatedB2, true
	}

	return sig
}
