package models

import "time"

// RawMeasurementModel refleja raw_measurements. Los nombres de índice coinciden
// con los de la BD para que AutoMigrate no cree nada nuevo. La FK de
// ingestion_batch_id (raw_measurements_ingestion_batch_id_fkey) existe en la BD
// y no se declara acá: GORM crearía otra con otro nombre.
type RawMeasurementModel struct {
	// ID es GENERATED ALWAYS AS IDENTITY: la BD lo genera y GORM nunca lo escribe.
	ID               int64     `gorm:"column:id;primaryKey;generated:identity always;<-:false"`
	MeasuringPointID int64     `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_raw_measurements_point_metric_ts,priority:1"`
	MetricID         uint64    `gorm:"column:metric_id;not null;uniqueIndex:ux_raw_measurements_point_metric_ts,priority:2"`
	Timestamp        time.Time `gorm:"column:timestamp;not null;uniqueIndex:ux_raw_measurements_point_metric_ts,priority:3"`
	Value            float64   `gorm:"column:value;not null"`
	Source           string    `gorm:"column:source;not null"`
	IngestionBatchID *int64    `gorm:"column:ingestion_batch_id;index:ix_raw_measurements_batch"`
}

func (RawMeasurementModel) TableName() string {
	return "raw_measurements"
}
