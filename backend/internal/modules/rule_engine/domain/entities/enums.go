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

// EpisodeEvent es lo que pasó con el episodio en una hora: lo abrió, lo cerró
// o ninguna de las dos (dentro o fuera de él).
type EpisodeEvent string

const (
	EpisodeEventNone    EpisodeEvent = "NONE"
	EpisodeEventOpened  EpisodeEvent = "OPENED"
	EpisodeEventCleared EpisodeEvent = "CLEARED"
)

// ReasonCode es el motivo por el que una hora tiene su computed_level.
type ReasonCode string

const (
	// ReasonLeakThreshold: I superó el corte de un nivel (el raw).
	ReasonLeakThreshold ReasonCode = "LEAK_THRESHOLD"
	// ReasonLeakPersistence: la racha subió un nivel sobre el raw.
	ReasonLeakPersistence ReasonCode = "LEAK_PERSISTENCE"
)

// PersistenceMode es cómo se cuenta la persistencia que escala un nivel.
type PersistenceMode string

const (
	// PersistenceStrict: racha de horas calculadas seguidas sobre el corte.
	PersistenceStrict PersistenceMode = "STRICT"
	// PersistenceWindow: horas calculadas sobre el corte dentro de una ventana móvil.
	PersistenceWindow PersistenceMode = "WINDOW"
)
