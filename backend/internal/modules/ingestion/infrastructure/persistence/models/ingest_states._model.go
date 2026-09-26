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
}
