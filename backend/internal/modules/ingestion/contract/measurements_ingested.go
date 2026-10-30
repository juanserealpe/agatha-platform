package contract

import "time"

// MeasurementsIngested indica que un lote de mediciones quedó persistido (post-commit).
//
// Es una PISTA: lleva referencias, no las mediciones. La fuente de verdad es la BD.
// Todos los time.Time están en UTC.
type MeasurementsIngested struct {
	// MeasuringPointID es el punto de medición (dispositivo) del lote.
	MeasuringPointID int64 `json:"measuring_point_id"`
	// Source es el origen del lote (p. ej. "dataos").
	Source string `json:"source"`
	// From es el timestamp mínimo de las mediciones insertadas.
	From time.Time `json:"from"`
	// To es el timestamp máximo de las mediciones insertadas.
	To time.Time `json:"to"`
	// Watermark es el last_ingested_at persistido para el lote.
	Watermark time.Time `json:"watermark"`
	// BatchSeq es el cursor monótono del lote (id de la fila en ingestion_state_models).
	BatchSeq int64 `json:"batch_seq"`
	// InsertedRecords es la cantidad de mediciones realmente insertadas.
	InsertedRecords int `json:"inserted_records"`
	// OccurredAt es el instante en que se generó el evento.
	OccurredAt time.Time `json:"occurred_at"`
	// SchemaVersion es la versión del esquema del evento (ver SchemaVersion).
	SchemaVersion int `json:"schema_version"`
}
