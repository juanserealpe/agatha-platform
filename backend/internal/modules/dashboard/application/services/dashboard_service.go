package services

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
)

type DashboardService struct {
	deviceRepository repositories.DeviceRepository
}

func NewDashboardService(
	deviceRepository repositories.DeviceRepository,
) *DashboardService {
	return &DashboardService{
		deviceRepository: deviceRepository,
	}
}

func (s *DashboardService) GetAllDevices(
	ctx context.Context,
) ([]dtos.DeviceResponse, error) {

	devices, err := s.deviceRepository.GetAllDeviceSummaries(ctx)
	if err != nil {
		return nil, err
	}

	response := make([]dtos.DeviceResponse, 0, len(devices))

	for _, device := range devices {
		response = append(response, dtos.DeviceResponse{
			ID:               device.ID,
			Name:             device.Name,
			LastConnection:   device.LastConnection,
			LastBatteryLevel: device.LastBatteryLevel,
		})
	}

	return response, nil
}
