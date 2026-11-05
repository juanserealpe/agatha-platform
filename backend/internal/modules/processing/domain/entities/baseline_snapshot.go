package entities

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// Valores de BaselineSnapshot que Processing interpreta.
const (
	ConfidenceNormal             = "NORMAL"
	ConfidenceLow                = "LOW"
	BaselineFlagSignalDegenerate = "SIGNAL_DEGENERATE"
)

// BaselineSnapshot es la vista propia de Processing de la línea base vigente
// en una hora. Usa los tipos del kernel compartido: es exactamente lo que
// reciben features.ComputeComponents y features.Score.
type BaselineSnapshot struct {
	// Available = false ⇒ no hay versión vigente (NO_SIGNAL_BASELINE). No es error.
	Available  bool
	VersionID  int64
	Version    int
	ValidFrom  time.Time
	ValidTo    *time.Time // nil ⇒ es la versión vigente hoy
	Confidence string     // ConfidenceNormal | ConfidenceLow
	// FirstSeenAt es el primer registro de B2 del dispositivo (nil si no hay).
	// Se informa aunque Available = false.
	FirstSeenAt      *time.Time
	Signal           features.Signal
	Stats            map[int]features.ComponentStats // clave: 6 | 24
	Flags            []string
	AlgorithmVersion string
}

// IsEffectiveAt indica si la versión está vigente en t: ValidFrom <= t < ValidTo.
func (s BaselineSnapshot) IsEffectiveAt(t time.Time) bool {
	if !s.Available || t.Before(s.ValidFrom) {
		return false
	}
	return s.ValidTo == nil || t.Before(*s.ValidTo)
}

// IsSignalDegenerate indica si la señal no tiene MinElevatedB2 (flag SIGNAL_DEGENERATE).
func (s BaselineSnapshot) IsSignalDegenerate() bool {
	if !s.Signal.HasMinElevated {
		return true
	}
	for _, f := range s.Flags {
		if f == BaselineFlagSignalDegenerate {
			return true
		}
	}
	return false
}
