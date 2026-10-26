// Package contract es la superficie pública del Rule Engine para otros
// módulos (Alert Management): tópicos y eventos. Solo depende de la stdlib.
//
// Clave de idempotencia para los consumidores: (MeasuringPointID, EventType,
// EvaluatedAt). Los eventos son best-effort: la fuente de verdad es la tabla
// rule_evaluations (las horas NO_CALCULABLE no tienen fila).
package contract

const (
	// TopicLevelEvaluated se publica por CADA hora de 24 h que llega en vivo:
	// las CALCULATED (incluidas las de nivel NORMAL) con su fila de
	// rule_evaluations, y las NO_CALCULABLE sin fila. No se publica en el
	// backfill, en llegadas tardías ni en duplicados.
	TopicLevelEvaluated = "rule_engine.level_evaluated"

	// SchemaVersion es la versión del esquema de los eventos de este contrato.
	// v2 reemplaza a leak_threshold_exceeded / leak_threshold_cleared (v1).
	// v3 agrega runs, escalation, below_run, d_pl y persistence_mode (aditivo:
	// los campos v2 no cambian).
	SchemaVersion = 3
)

// Status de la hora evaluada (los mismos valores que Processing).
const (
	StatusCalculated   = "CALCULATED"
	StatusNoCalculable = "NO_CALCULABLE"
)

// Niveles (orden NORMAL < AMARILLO < NARANJA < ROJO).
const (
	LevelNormal   = "NORMAL"
	LevelAmarillo = "AMARILLO"
	LevelNaranja  = "NARANJA"
	LevelRojo     = "ROJO"
)

// EpisodeEvent: qué pasó con el episodio del Rule Engine en la hora.
const (
	EpisodeEventOpened  = "OPENED"
	EpisodeEventCleared = "CLEARED"
	EpisodeEventNone    = "NONE"
)

// Códigos de Reason.
const (
	ReasonLeakThreshold   = "LEAK_THRESHOLD"
	ReasonLeakPersistence = "LEAK_PERSISTENCE"
	// ReasonNaranjaNotConfirmed: NARANJA no confirmado por planitud (la hora queda en AMARILLO).
	ReasonNaranjaNotConfirmed = "NARANJA_NOT_CONFIRMED"
)

// Modos de persistencia.
const (
	PersistenceStrict = "STRICT"
	PersistenceWindow = "WINDOW"
)
