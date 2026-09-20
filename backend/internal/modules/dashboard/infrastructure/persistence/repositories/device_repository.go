package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"

	"gorm.io/gorm"
)

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{
		db: db,
	}
}

func (r *DeviceRepository) GetAllDeviceSummaries(
	ctx context.Context,
) ([]entities.DeviceSummary, error) {

	var rows []models.DeviceSummaryModel

	err := r.db.WithContext(ctx).
		Table("devices AS d").
		Select(`
			d.measuring_point_id,
			d.name,
			i.last_ingested_at AS last_connection,
			b.value AS last_battery_level
		`).
		Joins(`
			LEFT JOIN LATERAL (
				SELECT i.last_ingested_at
				FROM ingestion_state_models AS i
				WHERE i.measuring_point_id = d.measuring_point_id
				ORDER BY i.last_ingested_at DESC
				LIMIT 1
			) AS i ON TRUE
		`).
		Joins(`
			LEFT JOIN LATERAL (
				SELECT rm.value
				FROM raw_measurements AS rm
				INNER JOIN metric_definitions AS md
					ON md.id = rm.metric_id
				WHERE rm.measuring_point_id = d.measuring_point_id
					AND md.key = 'bateria'
				ORDER BY rm.timestamp DESC
				LIMIT 1
			) AS b ON TRUE
		`).
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make([]entities.DeviceSummary, 0, len(rows))

	for _, row := range rows {
		result = append(result, mappers.ToDeviceSummary(row))
	}

	return result, nil
}

func (r *DeviceRepository) ExistsDevice(ctx context.Context, deviceID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("devices").
		Where("id = ?", deviceID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
