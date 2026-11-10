package entities

// Valores que llegan de Alert Management (en español). El módulo no importa su
// contrato: internal/bootstrap traduce los eventos a AlertNotice y AlertChange.
const (
	LevelAmarillo = "AMARILLO"
	LevelNaranja  = "NARANJA"
	LevelRojo     = "ROJO"

	EventLeak     = "FUGA"
	EventMovement = "MOVIMIENTO"

	ClassificationConfirmed  = "CONFIRMADA"
	ClassificationFalseAlarm = "FALSA_ALERTA"
)

// PushKind dice a la app qué texto mostrar (README.md §5.3).
type PushKind string

const (
	PushKindNew     PushKind = "NEW"      // alerta generada
	PushKindLevelUp PushKind = "LEVEL_UP" // subió de nivel
)

// PushLevel es el nivel en inglés que espera la app.
type PushLevel string

const (
	PushLevelYellow PushLevel = "YELLOW"
	PushLevelOrange PushLevel = "ORANGE"
	PushLevelRed    PushLevel = "RED"
)

// PushEvent es el tipo de evento en inglés que espera la app.
type PushEvent string

const (
	PushEventLeak     PushEvent = "LEAK"
	PushEventMovement PushEvent = "MOVEMENT"
)

// PushClassification es la clasificación en inglés que espera la app.
type PushClassification string

const (
	PushClassificationConfirmed  PushClassification = "CONFIRMED"
	PushClassificationFalseAlarm PushClassification = "FALSE_ALARM"
)

// PushKindFor traduce el código del catálogo de mensajes. false = el código no
// genera push (o es desconocido).
func PushKindFor(code string) (PushKind, bool) {
	switch code {
	case "OPEN", "NEW_AFTER_CLASSIFIED":
		return PushKindNew, true
	case "UP_NARANJA_PERSIST", "UP_NARANJA_FAST", "UP_ROJO", "REACTIVATED":
		return PushKindLevelUp, true
	default:
		return "", false
	}
}

// PushLevelFor traduce el nivel. false = NORMAL o desconocido (no se notifica).
func PushLevelFor(level string) (PushLevel, bool) {
	switch level {
	case LevelAmarillo:
		return PushLevelYellow, true
	case LevelNaranja:
		return PushLevelOrange, true
	case LevelRojo:
		return PushLevelRed, true
	default:
		return "", false
	}
}

// PushEventFor traduce el tipo de evento. false = desconocido (se omite).
func PushEventFor(event string) (PushEvent, bool) {
	switch event {
	case EventLeak:
		return PushEventLeak, true
	case EventMovement:
		return PushEventMovement, true
	default:
		return "", false
	}
}

// PushClassificationFor traduce la clasificación. false = sin clasificar (se omite).
func PushClassificationFor(classification string) (PushClassification, bool) {
	switch classification {
	case ClassificationConfirmed:
		return PushClassificationConfirmed, true
	case ClassificationFalseAlarm:
		return PushClassificationFalseAlarm, true
	default:
		return "", false
	}
}
