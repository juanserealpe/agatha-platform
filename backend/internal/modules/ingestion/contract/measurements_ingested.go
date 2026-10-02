package contract

// MeasurementsIngested indica que un lote de mediciones quedó persistido (post-commit).
//
// Es una PISTA: lleva referencias, no las mediciones. La fuente de verdad es la BD.
// Todos los time.Time están en UTC.
type MeasurementsIngested struct {
	// MeasuringPointID es el punto de medición (dispositivo) del lote.
	MeasuringPointID int64 `json:"measuring_point_id"`
	// InsertedRecords es la cantidad de mediciones realmente insertadas.
	InsertedRecords int `json:"inserted_records"`
	// SchemaVersion es la versión del esquema del evento (ver SchemaVersion).
	SchemaVersion int `json:"schema_version"`
}
