// Package contract es la superficie pública del Rule Engine para otros
// módulos (Alert Management): tópicos y eventos. Solo depende de la stdlib.
//
// Clave de idempotencia para los consumidores: (MeasuringPointID, EventType,
// EvaluatedAt). Los eventos son best-effort: la fuente de verdad es la tabla
// rule_evaluations.
package contract

const (
	// TopicLeakThresholdExceeded se publica por cada hora calculada en la que
	// un episodio de fuga está por encima del umbral (rule_evaluations.emitted = EXCEEDED).
	TopicLeakThresholdExceeded = "rule_engine.leak_threshold_exceeded"

	// TopicLeakThresholdCleared se publica UNA vez, en la hora que cierra el
	// episodio (rule_evaluations.emitted = CLEARED).
	TopicLeakThresholdCleared = "rule_engine.leak_threshold_cleared"

	// SchemaVersion es la versión del esquema de los eventos de este contrato.
	SchemaVersion = 1
)
