// Package contract es la superficie pública de Ingestion para otros módulos:
// tópicos, eventos y puertos de consulta. Solo depende de la stdlib.
package contract

const (
	// TopicMeasurementsIngested se publica tras cada lote de mediciones persistido.
	TopicMeasurementsIngested = "ingestion.measurements_ingested"

	// SchemaVersion es la versión del esquema de los eventos de este contrato.
	SchemaVersion = 1
)
