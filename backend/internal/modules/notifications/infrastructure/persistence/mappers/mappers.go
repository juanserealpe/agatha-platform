// Package mappers traduce entre las entidades de Notifications y sus filas.
package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/models"
)

func ToPushDeviceModel(d entities.PushDevice) models.PushDeviceModel {
	return models.PushDeviceModel{
		DeviceID:  d.DeviceID,
		Token:     d.Token,
		Platform:  string(d.Platform),
		UpdatedAt: d.UpdatedAt.UTC(),
	}
}

func ToPushDevice(m models.PushDeviceModel) entities.PushDevice {
	return entities.PushDevice{
		DeviceID:  m.DeviceID,
		Token:     m.Token,
		Platform:  entities.PushPlatform(m.Platform),
		UpdatedAt: m.UpdatedAt.UTC(),
	}
}
