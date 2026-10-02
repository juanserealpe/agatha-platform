package entities

import "time"

// LatestIngestion es la última fecha ingerida de un punto de medición
// con todas las variables registradas en esa fecha.
type LatestIngestion struct {
	MeasuringPointID int64
	Timestamp        time.Time
	Measurements     []RawMeasurement
}
