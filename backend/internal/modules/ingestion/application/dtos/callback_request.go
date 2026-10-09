package dtos

import (
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
)

// DataRecordCallback es el cuerpo que envía Dataflow Labs a POST /ingestion/callback.
// Los timestamps se reciben como texto y se validan en CallbackValidator.
type DataRecordCallback struct {
	Event     string            `json:"event"`
	RecordID  string            `json:"record_id"`
	MpID      int64             `json:"mp_id"`
	SerialID  string            `json:"serial_id"`
	Timestamp string            `json:"timestamp"`
	Records   []CallbackReading `json:"records"`
}

// CallbackReading es una variable dentro de records.
type CallbackReading struct {
	DeviceID  int64   `json:"device_id"`
	Timestamp string  `json:"timestamp"`
	Name      string  `json:"name"`
	Readings  Reading `json:"readings"`
}

// Reading conserva el literal numérico JSON tal como llegó (22.4, 0.0, 11556)
// para no perder precisión al convertir. Vacío = campo ausente.
type Reading string

// UnmarshalJSON acepta solo números JSON: rechaza strings, booleanos, null,
// objetos y arreglos. La validez del literal ya la garantizó el decoder.
func (r *Reading) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || (data[0] != '-' && (data[0] < '0' || data[0] > '9')) {
		return ingestionErrors.NewInvalidCallbackPayloadError("readings debe ser un número")
	}

	*r = Reading(data)
	return nil
}
