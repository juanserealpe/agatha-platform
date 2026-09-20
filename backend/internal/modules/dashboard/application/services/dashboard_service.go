package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type DashboardService struct {
	deviceRepository      repositories.DeviceRepository
	measurementRepository repositories.MeasurementRepository
	dashboardLogger       *logger.Logger
}

func NewDashboardService(
	deviceRepository repositories.DeviceRepository,
	measurementRepository repositories.MeasurementRepository,
	dashboardLogger *logger.Logger,
) *DashboardService {
	return &DashboardService{
		deviceRepository:      deviceRepository,
		measurementRepository: measurementRepository,
		dashboardLogger:       dashboardLogger,
	}
}

func (s *DashboardService) GetAllDevices(ctx context.Context,
) ([]dtos.DeviceResponse, error) {
	s.dashboardLogger.Info("[DashboardService] Obteniendo todos los dispositivos")
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
		s.dashboardLogger.Info("[DashboardService] Dispositivo recibido: ID=%d, Name=%s", device.ID, device.Name)
	}
	s.dashboardLogger.Info("[DashboardService] Total de dispositivos obtenidos: %d", len(response))
	return response, nil
}

func (s *DashboardService) GetMeasurements(ctx context.Context, deviceID int64, startDate time.Time, endDate time.Time,
) (*dtos.MeasurementsResponse, error) {

	existDeviceResponse, err := s.deviceRepository.ExistsDevice(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("error checking device existence: %w", err)
	}

	if !existDeviceResponse {
		return nil, fmt.Errorf("device with ID %d does not exist", deviceID)
	}

	s.dashboardLogger.Info("[DashboardService] Obteniendo mediciones para el dispositivo ID=%d desde %s hasta %s", deviceID, startDate.Format(time.RFC3339), endDate.Format(time.RFC3339))
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
