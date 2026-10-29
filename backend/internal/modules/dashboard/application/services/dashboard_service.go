package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type DashboardService struct {
	deviceRepository      repositories.DeviceRepository
	measurementRepository repositories.MeasurementRepository
	dashboardValidator    *DashboardValidator
	dashboardLogger       *logger.Logger
}

func NewDashboardService(
	deviceRepository repositories.DeviceRepository,
	measurementRepository repositories.MeasurementRepository,
	dashboardValidator *DashboardValidator,
	dashboardLogger *logger.Logger,
) *DashboardService {
	return &DashboardService{
		deviceRepository:      deviceRepository,
		measurementRepository: measurementRepository,
		dashboardValidator:    dashboardValidator,
		dashboardLogger:       dashboardLogger,
	}
}

func (s *DashboardService) GetAllDevices(ctx context.Context,
) ([]dtos.DeviceResponse, error) {
	s.dashboardLogger.Info("[DashboardService] Obteniendo todos los dispositivos")
	devices, err := s.deviceRepository.GetAllDeviceSummaries(ctx)
	if err != nil {
		s.dashboardLogger.Error("[DashboardService] Error consultando dispositivos", "error", err)
		return nil, domain.ErrConsultingDevices
	}

	response := make([]dtos.DeviceResponse, 0, len(devices))

	for _, device := range devices {
		response = append(response, dtos.DeviceResponse{
			ID:               device.ID,
			Name:             device.Name,
			LastConnection:   device.LastConnection,
			LastBatteryLevel: device.LastBatteryLevel,
		})
		s.dashboardLogger.Info("[DashboardService] Dispositivo recibido", "device_id", device.ID, "device_name", device.Name)
	}
	s.dashboardLogger.Info("[DashboardService] Total de dispositivos obtenidos", "total", len(response))
	return response, nil
}

func (s *DashboardService) GetMeasurements(ctx context.Context, deviceID int64, startDate time.Time, endDate time.Time,
) (*dtos.MeasurementsResponse, error) {
	err := s.dashboardValidator.ValidateDateRange(startDate, endDate)
	if err != nil {
		return nil, err
	}

	err = s.dashboardValidator.ValidateNotFuture(startDate, domain.ErrStartDateFuture)
	if err != nil {
		return nil, err
	}

	err = s.dashboardValidator.ValidateNotFuture(endDate, domain.ErrEndDateFuture)
	if err != nil {
		return nil, err
	}

	existDeviceResponse, err := s.deviceRepository.ExistsDevice(ctx, deviceID)

	if err != nil {
		s.dashboardLogger.Error(
			"[DashboardService] Error verificando existencia del dispositivo",
			"device_id", deviceID,
			"error", err,
		)

		return nil, domain.ErrConsultingMeasurements
	}

	if !existDeviceResponse {
		s.dashboardLogger.Error(
			"[DashboardService] Dispositivo no encontrado",
			"device_id", deviceID,
		)

		return nil, appErrors.ErrDeviceNotFound
	}

	s.dashboardLogger.Info(
		"[DashboardService] Dispositivo encontrado",
		"device_id", deviceID,
	)

	s.dashboardLogger.Info(
		"[DashboardService] Obteniendo mediciones para el dispositivo",
		"device_id", deviceID,
		"start_date", startDate.Format(time.RFC3339),
		"end_date", endDate.Format(time.RFC3339),
	)

	measurements, err := s.measurementRepository.GetMeasurements(
		ctx,
		deviceID,
		startDate,
		endDate,
	)

	if err != nil {
		s.dashboardLogger.Error(
			"[DashboardService] Error consultando mediciones",
			"device_id", deviceID,
			"error", err,
		)
		return nil, domain.ErrConsultingMeasurements
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
