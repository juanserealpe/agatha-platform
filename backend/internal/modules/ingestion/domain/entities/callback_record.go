package entities

import "time"

// CallbackRecord es un registro de Dataflow Labs ya validado. RecordID es
// estable entre reintentos y se usa para deduplicar.
type CallbackRecord struct {
	RecordID         string
	MeasuringPointID int64
	SerialID         string
	Timestamp        time.Time
	Readings         []CallbackReading
}

// CallbackReading es una variable del registro. Value es el literal numérico
// JSON tal como llegó (entero o decimal), sin conversión que pierda precisión.
// RawTimestamp conserva el texto original del timestamp.
type CallbackReading struct {
	DeviceID     int64
	Name         string
	Timestamp    time.Time
	RawTimestamp string
	Value        string
}

// LatestReadingTimestamp devuelve el MAX(timestamp) de las variables, o el
// timestamp del registro si no hay variables.
func (r *CallbackRecord) LatestReadingTimestamp() time.Time {
	latest := r.Timestamp
	for i, reading := range r.Readings {
		if i == 0 || reading.Timestamp.After(latest) {
			latest = reading.Timestamp
		}
	}
	return latest
}
