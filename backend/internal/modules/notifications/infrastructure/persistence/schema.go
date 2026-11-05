// Package persistence agrupa el esquema de Notifications: la tabla
// push_devices (tokens FCM de los celulares).
package persistence

import (
	"fmt"

	"gorm.io/gorm"
)

// Migrate crea push_devices. Es idempotente y solo aditivo: corre en cada
// arranque contra la base del piloto, así que no usa AutoMigrate (que podría
// alterar columnas) ni toca otras tablas.
//
// El token no lleva índice único directo: un token de hasta 4096 bytes supera
// el tope de una entrada btree (~2700 bytes). La unicidad va sobre md5(token),
// y las consultas por token filtran por md5(token) y por token.
func Migrate(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS push_devices (
			device_id  text PRIMARY KEY CHECK (char_length(device_id) BETWEEN 1 AND 128),
			token      text NOT NULL CHECK (char_length(token) BETWEEN 1 AND 4096),
			platform   text NOT NULL CHECK (platform IN ('ANDROID')),
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_push_devices_token ON push_devices (md5(token))`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrar notifications: %w", err)
		}
	}
	return nil
}
