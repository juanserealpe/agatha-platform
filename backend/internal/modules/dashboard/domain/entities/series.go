package entities

import "time"

// SeriesSource indica de qué tabla sale una variable graficable.
type SeriesSource string

const (
	// SourceMeasurement: telemetría cruda (raw_measurements), por metric_id.
	SourceMeasurement SeriesSource = "measurement"
	// SourceEvaluation: resultados horarios de Processing (feature_evaluations), por columna.
	SourceEvaluation SeriesSource = "evaluation"
)

// SeriesDefinition describe una variable que el dashboard puede graficar en
// el tiempo. Según Source se usa MetricID (measurement) o Column + WindowHours
// (evaluation).
type SeriesDefinition struct {
	Key    string // identificador estable en la API (query param y respuesta)
	Name   string
	Unit   string
	Source SeriesSource

	MetricID uint64 // SourceMeasurement: id del catálogo de métricas

	Column      string // SourceEvaluation: columna de feature_evaluations
	WindowHours int    // SourceEvaluation: 6 | 24
}

// SeriesPoint es un valor de una variable en un instante.
type SeriesPoint struct {
	Timestamp time.Time
	Value     float64
}
