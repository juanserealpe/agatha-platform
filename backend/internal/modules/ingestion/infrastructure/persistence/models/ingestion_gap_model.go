package models

import (
	"time"
)

type IngestionGapModel struct {
	ID               uint64     `gorm:"primaryKey"`
	MeasuringPointID uint64     `gorm:"not null;index"`
	StartedAt        time.Time  `gorm:"not null;index"`
	EndedAt          *time.Time `gorm:"index"`
	ExpectedRecords  int        `gorm:"not null"`
	ReceivedRecords  int        `gorm:"not null"`
	Status           string     `gorm:"not null;index"`
	CreatedAt        time.Time  `gorm:"not null"`
	ResolvedAt       *time.Time `gorm:"index"`
}

func (IngestionGapModel) TableName() string {
	return "ingestion_gap_models"
}
