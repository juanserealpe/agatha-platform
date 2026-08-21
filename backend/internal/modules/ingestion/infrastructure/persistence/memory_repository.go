package persistence

import (
	"fmt"
	"sync"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
)

type MemoryRepository struct {
	mu   sync.RWMutex
	data []*entities.RawMeasurement
}

func NewMemoryRepository() repositories.MeasurementRepository {
	return &MemoryRepository{
		data: make([]*entities.RawMeasurement, 0),
	}
}

func (r *MemoryRepository) Save(measurement *entities.RawMeasurement) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	measurement.ID = fmt.Sprintf("meas-%d", len(r.data)+1)
	r.data = append(r.data, measurement)
	return nil
}

func (r *MemoryRepository) FindByDeviceID(deviceID string, limit int) ([]*entities.RawMeasurement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*entities.RawMeasurement
	for _, m := range r.data {
		if m.DeviceID == deviceID {
			result = append(result, m)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}
