package services

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type DashboardService struct {
	deviceRepository repositories.DeviceRepository
	dashboardLogger  *logger.Logger
}

func NewDashboardService(
	deviceRepository repositories.DeviceRepository,
	dashboardLogger *logger.Logger,
) *DashboardService {
	return &DashboardService{
		deviceRepository: deviceRepository,
		dashboardLogger:  dashboardLogger,
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
		s.dashboardLogger.Info("dispositivo obtenido", "device_id", device.ID, "device_name", device.Name,
			"last_connection", device.LastConnection, "last_battery_level", device.LastBatteryLevel)
	}

	return response, nil
}
