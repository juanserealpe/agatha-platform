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
	VersionID  int64 // 0 en las versiones simuladas por el replay
	Version    int
	ValidFrom  time.Time
	Confidence string // ConfidenceNormal | ConfidenceLow
	// FirstSeenAt es el primer registro de B2 del dispositivo (nil si no hay).
	// Se informa aunque Available = false.
	FirstSeenAt      *time.Time
	Signal           features.Signal
	Stats            map[int]features.ComponentStats // clave: 6 | 24
	Flags            []string
	AlgorithmVersion string
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
