package models

import "time"

// RawSampleRowModel es la vista propia (solo lectura) de raw_measurements que
// usa Baseline. La tabla la crea y escribe Ingestion: NO se migra acá.
type RawSampleRowModel struct {
	Timestamp time.Time `gorm:"column:timestamp"`
	MetricID  uint64    `gorm:"column:metric_id"`
	Value     float64   `gorm:"column:value"`
}
