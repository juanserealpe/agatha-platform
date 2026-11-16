package database

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

// Una consulta que falla se registra sin sus valores: un INSERT en push_devices
// no puede dejar el token FCM completo en el log (pasó el 13/11/2026).
func TestNewGormLogger_NeverLogsQueryValues(t *testing.T) {
	filter, ok := newGormLogger().(gorm.ParamsFilter)
	if !ok {
		t.Fatal("el logger de GORM debe filtrar los parámetros de las consultas")
	}
	sql, params := filter.ParamsFilter(context.Background(),
		"INSERT INTO push_devices (device_id, token) VALUES ($1, $2)", "CEL-1A2B3C", "token-secreto")
	if params != nil {
		t.Fatalf("parámetros en el log = %v, se esperaba nil", params)
	}
	if sql != "INSERT INTO push_devices (device_id, token) VALUES ($1, $2)" {
		t.Fatalf("sql = %q", sql)
	}
}
