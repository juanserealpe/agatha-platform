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
	return &DeviceRepository{
		db: db,
	}
}

func (r *DeviceRepository) Save(
	ctx context.Context,
	entity entities.Device,
) error {
	model := mappers.ToDeviceModel(&entity)

	return r.db.
		WithContext(ctx).
		Create(model).
		Error
}

func (r *DeviceRepository) FindByID(
	ctx context.Context,
	id string,
) (*entities.Device, error) {
	var model models.DeviceModel

	err := r.db.
		WithContext(ctx).
		First(&model, "id = ?", id).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return mappers.ToDeviceEntity(&model), nil
}

func (r *DeviceRepository) FindAll(
	ctx context.Context,
) ([]*entities.Device, error) {
	var modelsList []models.DeviceModel

	err := r.db.
		WithContext(ctx).
		Find(&modelsList).
		Error

	if err != nil {
		return nil, err
	}

	entitiesList := make([]*entities.Device, 0, len(modelsList))

	for i := range modelsList {
		entitiesList = append(
			entitiesList,
			mappers.ToDeviceEntity(&modelsList[i]),
		)
	}

	return entitiesList, nil
}

func (r *DeviceRepository) Update(
	ctx context.Context,
	entity *entities.Device,
) error {
	model := mappers.ToDeviceModel(entity)

	return r.db.
		WithContext(ctx).
		Model(&models.DeviceModel{}).
		Where("id = ?", entity.ID).
		Updates(model).
		Error
}

func (r *DeviceRepository) Delete(
	ctx context.Context,
	id string,
) error {
	return r.db.
		WithContext(ctx).
		Delete(&models.DeviceModel{}, "id = ?", id).
		Error
}

func (r *DeviceRepository) FindByMeasuringPointID(
	ctx context.Context,
	measuringPointID int64,
) (*entities.Device, error) {

	var model models.DeviceModel

	err := r.db.
		WithContext(ctx).
		Where("measuring_point_id = ?", measuringPointID).
		First(&model).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return mappers.ToDeviceEntity(&model), nil
}

func (r *DeviceRepository) List(
	ctx context.Context,
) ([]entities.Device, error) {

	var modelsList []models.DeviceModel

	if err := r.db.
		WithContext(ctx).
		Find(&modelsList).
		Error; err != nil {
		return nil, err
	}

	result := make([]entities.Device, 0, len(modelsList))

	for i := range modelsList {
		entity := mappers.ToDeviceEntity(&modelsList[i])

		if entity != nil {
			result = append(result, *entity)
		}
	}

	return result, nil
}
