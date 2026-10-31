// Package models son las filas de las tablas de Notifications. El esquema lo
// crea persistence.Migrate con SQL (no AutoMigrate).
package models

import "time"

// PushDeviceModel es el token vigente de un celular. created_at lo pone la
// base y no se lee.
type PushDeviceModel struct {
	DeviceID  string    `gorm:"column:device_id;primaryKey"`
	Token     string    `gorm:"column:token"`
	Platform  string    `gorm:"column:platform"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (PushDeviceModel) TableName() string { return "push_devices" }
