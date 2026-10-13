package dtos

import "time"

// SeriesResponse son las variables pedidas de un dispositivo en un rango.
type SeriesResponse struct {
	DeviceID  int64                    `json:"deviceId"`
	StartDate time.Time                `json:"startDate"`
	EndDate   time.Time                `json:"endDate"`
	Series    []VariableSeriesResponse `json:"series"`
}

// VariableSeriesResponse es una variable con sus puntos. Una variable sin
// datos en el rango viene con data vacío.
type VariableSeriesResponse struct {
	Key    string                     `json:"key"`
	Name   string                     `json:"name"`
	Unit   string                     `json:"unit"`
	Source string                     `json:"source"`
	Data   []MeasurementPointResponse `json:"data"`
}

// VariableResponse describe una variable disponible (sin datos).
type VariableResponse struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Unit   string `json:"unit"`
	Source string `json:"source"`
}
