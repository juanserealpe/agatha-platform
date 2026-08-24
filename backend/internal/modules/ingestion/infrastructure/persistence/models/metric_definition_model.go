package models

import "time"

type MetricDefinitionModel struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Key       string    `gorm:"column:key;not null;uniqueIndex"`
	Name      string    `gorm:"column:name;not null"`
	Unit      string    `gorm:"column:unit;not null"`
	Category  string    `gorm:"column:category;not null"`
	Active    bool      `gorm:"column:active;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (MetricDefinitionModel) TableName() string {
	return "metric_definitions"
}
