package models

import "time"

type IngestionStateModel struct {
	ID               uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID uint64     `gorm:"column:measuring_point_id;not null"`
	Source           string     `gorm:"column:source;not null"`
	LastIngestedAt   *time.Time `gorm:"column:last_ingested_at"`
	ReceivedRecords  int        `gorm:"column:received_records;not null"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null"`
	// RecordID es único cuando no es NULL (PostgreSQL admite varios NULL):
	// deduplica los reintentos del callback de forma segura ante concurrencia.
	RecordID *string `gorm:"column:record_id;size:128;uniqueIndex:ux_ingestion_logs_record_id"`
}
