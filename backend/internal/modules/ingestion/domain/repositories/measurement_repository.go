package repositories

import "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"

type MeasurementRepository interface {
	Save(measurement *entities.RawMeasurement) error
	FindByDeviceID(deviceID string, limit int) ([]*entities.RawMeasurement, error)
}
