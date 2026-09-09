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
			LEFT JOIN ingestion_states AS i
				ON i.measuring_point_id = d.measuring_point_id
		`).
		Joins(`
			LEFT JOIN LATERAL (
				SELECT rm.value
				FROM raw_measurements AS rm
				INNER JOIN metric_definitions AS md
					ON md.id = rm.metric_id
				WHERE rm.measuring_point_id = d.measuring_point_id
					AND md.key = 'battery_level'
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
