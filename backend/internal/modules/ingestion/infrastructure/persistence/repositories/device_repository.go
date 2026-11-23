package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) FindByMeasuringPointID(ctx context.Context, measuringPointID int64,
) (*entities.Device, error) {
	var model models.DeviceModel

	err := r.db.WithContext(ctx).Where("measuring_point_id = ?", measuringPointID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return mappers.ToDeviceEntity(&model), nil
}

func (r *DeviceRepository) FindActive(ctx context.Context) ([]entities.Device, error) {
	var rows []models.DeviceModel
	if err := r.db.WithContext(ctx).Where("active = ?", true).Order("measuring_point_id").Find(&rows).Error; err != nil {
		return nil, err
	}

	devices := make([]entities.Device, 0, len(rows))
	for i := range rows {
		devices = append(devices, *mappers.ToDeviceEntity(&rows[i]))
	}
	return devices, nil
}
