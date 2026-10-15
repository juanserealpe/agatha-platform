// Package contract es la superficie pública de Processing para otros módulos:
// tópicos y eventos. Solo depende de la stdlib.
package contract

const (
	// TopicIndicatorEvaluated se publica por cada ventana (24 h y 6 h) evaluada
	// y persistida en feature_evaluations.
	TopicIndicatorEvaluated = "processing.indicator_evaluated"

	// SchemaVersion es la versión del esquema de los eventos de este contrato.
	SchemaVersion = 1
)
