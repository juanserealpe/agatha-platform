// Package entities define el modelo del Rule Engine: umbrales versionados por
// punto y la interpretación horaria del índice I. Solo stdlib, sin IO.
package entities

// EventType es el tipo de evento que interpretan los umbrales. Hoy solo FUGA.
type EventType string

const EventTypeLeak EventType = "FUGA"

// Level es el nivel interpretado de una hora. Orden: NORMAL < AMARILLO < NARANJA < ROJO.
type Level string

const (
	LevelNormal   Level = "NORMAL"
	LevelAmarillo Level = "AMARILLO"
	LevelNaranja  Level = "NARANJA"
	LevelRojo     Level = "ROJO"
)

// Rank devuelve la posición del nivel en el orden (NORMAL = 0). Un nivel
// desconocido cuenta como NORMAL.
func (l Level) Rank() int {
	switch l {
	case LevelAmarillo:
		return 1
	case LevelNaranja:
		return 2
	case LevelRojo:
		return 3
	default:
		return 0
	}
}

// AtLeast indica si l >= other en el orden de niveles.
func (l Level) AtLeast(other Level) bool { return l.Rank() >= other.Rank() }

// MaxLevel devuelve el mayor de dos niveles.
func MaxLevel(a, b Level) Level {
	if b.Rank() > a.Rank() {
		return b
	}
	return a
}

// Rule es la regla que fijó el nivel de una hora.
type Rule string

const (
	// RuleThreshold: el nivel es el que marca el umbral (raw).
	RuleThreshold Rule = "THRESHOLD"
	// RulePersistence: la racha subió el nivel un escalón sobre el raw.
	RulePersistence Rule = "PERSISTENCE"
	// RuleHold: dentro de un episodio, I bajó del umbral pero aún no se cumple la salida (histéresis).
	RuleHold Rule = "HOLD"
	// RuleNone: sin regla activa (fuera de episodio o al cerrarlo).
	RuleNone Rule = "NONE"
)

// Emission es el evento que corresponde publicar por una hora.
type Emission string

const (
	EmittedNone     Emission = "NONE"
	EmittedExceeded Emission = "EXCEEDED"
	EmittedCleared  Emission = "CLEARED"
)
