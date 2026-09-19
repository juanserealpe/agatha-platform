package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type DashboardService struct {
	deviceRepository      repositories.DeviceRepository
	dashboardLogger       *logger.Logger
	measurementRepository repositories.MeasurementRepository
}

func NewDashboardService(
	deviceRepository repositories.DeviceRepository,
	dashboardLogger *logger.Logger,
	measurementRepository repositories.MeasurementRepository,
) *DashboardService {
	return &DashboardService{
		deviceRepository:      deviceRepository,
		dashboardLogger:       dashboardLogger,
		measurementRepository: measurementRepository,
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

func (s *DashboardService) GetMeasurements(
	ctx context.Context,
	deviceID int64,
	startDate time.Time,
	endDate time.Time,
) (*dtos.MeasurementsResponse, error) {

	measurements, err := s.measurementRepository.GetMeasurements(
		ctx,
		deviceID,
		startDate,
		endDate,
	)

	if err != nil {
		return nil, err
	}

	metrics := make(map[uint64]*dtos.MetricMeasurementsResponse)

	for _, measurement := range measurements {

		metric, exists := metrics[measurement.MetricID]

		if !exists {
			metric = &dtos.MetricMeasurementsResponse{
				MetricID:   measurement.MetricID,
				MetricName: measurement.MetricName,
				Unit:       measurement.Unit,
				Data:       make([]dtos.MeasurementPointResponse, 0),
			}

			metrics[measurement.MetricID] = metric
		}

		metric.Data = append(
			metric.Data,
			dtos.MeasurementPointResponse{
				Timestamp: measurement.Timestamp,
				Value:     measurement.Value,
			},
		)
	}

	response := dtos.MeasurementsResponse{
		DeviceID:     deviceID,
		StartDate:    startDate,
		EndDate:      endDate,
		Measurements: make([]dtos.MetricMeasurementsResponse, 0, len(metrics)),
	}

	for _, metric := range metrics {
		response.Measurements = append(
			response.Measurements,
			*metric,
		)
	}

	return &response, nil
}
