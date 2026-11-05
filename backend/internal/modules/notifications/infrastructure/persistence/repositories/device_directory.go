package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/models"
)

// DeviceDirectory lee devices (de Ingestion, solo lectura y sin importar el
// módulo). Implementa ports.DeviceDirectory.
type DeviceDirectory struct {
	db *gorm.DB
}

func NewDeviceDirectory(db *gorm.DB) *DeviceDirectory {
	return &DeviceDirectory{db: db}
}

// DeviceName devuelve devices.name del punto, o vacío si el punto o la tabla
// no existen (el push se envía sin sensorId ni pk).
func (d *DeviceDirectory) DeviceName(ctx context.Context, measuringPointID int64) (string, error) {
	if !d.db.Migrator().HasTable("devices") {
		return "", nil
	}
	var row models.DeviceReadModel
	err := d.db.WithContext(ctx).Table("devices").Select("measuring_point_id, name").
		Where("measuring_point_id = ?", measuringPointID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Name, nil
}
