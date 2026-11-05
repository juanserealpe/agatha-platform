package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// SeriesService entrega las variables graficables de un dispositivo. No conoce
// ninguna variable en particular: las toma del catálogo (entities.SeriesCatalog)
// y delega la lectura en el lector de la fuente de cada una.
type SeriesService struct {
	deviceRepository   repositories.DeviceRepository
	readers            map[entities.SeriesSource]repositories.SeriesReader
	dashboardValidator *DashboardValidator
	dashboardLogger    *logger.Logger
}

func NewSeriesService(
	deviceRepository repositories.DeviceRepository,
	readers map[entities.SeriesSource]repositories.SeriesReader,
	dashboardValidator *DashboardValidator,
	dashboardLogger *logger.Logger,
) *SeriesService {
	return &SeriesService{
		deviceRepository:   deviceRepository,
		readers:            readers,
		dashboardValidator: dashboardValidator,
		dashboardLogger:    dashboardLogger,
	}
}

// GetVariables devuelve el catálogo de variables disponibles, en su orden.
func (s *SeriesService) GetVariables() []dtos.VariableResponse {
	catalog := entities.SeriesCatalog()

	response := make([]dtos.VariableResponse, 0, len(catalog))
	for _, definition := range catalog {
		response = append(response, dtos.VariableResponse{
			Key:    definition.Key,
			Name:   definition.Name,
			Unit:   definition.Unit,
			Source: string(definition.Source),
		})
	}

	return response
}

// GetSeries devuelve los puntos de las variables pedidas en [startDate,
// endDate). keys vacío ⇒ todas las del catálogo. La respuesta sigue el orden
// del catálogo.
func (s *SeriesService) GetSeries(
	ctx context.Context,
	deviceID int64,
	startDate time.Time,
	endDate time.Time,
	keys []string,
) (*dtos.SeriesResponse, error) {
	if err := s.dashboardValidator.ValidateDateRange(startDate, endDate); err != nil {
		return nil, err
	}
	if err := s.dashboardValidator.ValidateNotFuture(startDate, domain.ErrStartDateFuture); err != nil {
		return nil, err
	}
	if err := s.dashboardValidator.ValidateNotFuture(endDate, domain.ErrEndDateFuture); err != nil {
		return nil, err
	}

	definitions, unknown, ok := entities.ResolveSeries(keys)
	if !ok {
		s.dashboardLogger.Error("[SeriesService] Variable desconocida", "variable", unknown)
		return nil, domain.ErrUnknownVariable
	}

	exists, err := s.deviceRepository.ExistsDevice(ctx, deviceID)
	if err != nil {
		s.dashboardLogger.Error("[SeriesService] Error verificando existencia del dispositivo",
			"device_id", deviceID, "error", err)
		return nil, domain.ErrConsultingSeries
	}
	if !exists {
		s.dashboardLogger.Error("[SeriesService] Dispositivo no encontrado", "device_id", deviceID)
		return nil, appErrors.ErrDeviceNotFound
	}

	points, err := s.read(ctx, deviceID, definitions, startDate, endDate)
	if err != nil {
		return nil, err
	}

	response := dtos.SeriesResponse{
		DeviceID:  deviceID,
		StartDate: startDate,
		EndDate:   endDate,
		Series:    make([]dtos.VariableSeriesResponse, 0, len(definitions)),
	}

	for _, definition := range definitions {
		data := make([]dtos.MeasurementPointResponse, 0, len(points[definition.Key]))
		for _, point := range points[definition.Key] {
			data = append(data, dtos.MeasurementPointResponse{Timestamp: point.Timestamp, Value: point.Value})
		}

		response.Series = append(response.Series, dtos.VariableSeriesResponse{
			Key:    definition.Key,
			Name:   definition.Name,
			Unit:   definition.Unit,
			Source: string(definition.Source),
			Data:   data,
		})
	}

	s.dashboardLogger.Info("[SeriesService] Series obtenidas",
		"device_id", deviceID,
		"start_date", startDate.Format(time.RFC3339),
		"end_date", endDate.Format(time.RFC3339),
		"variables", len(definitions),
	)

	return &response, nil
}

// read agrupa las definiciones por fuente y hace una lectura por fuente.
func (s *SeriesService) read(
	ctx context.Context,
	deviceID int64,
	definitions []entities.SeriesDefinition,
	startDate time.Time,
	endDate time.Time,
) (map[string][]entities.SeriesPoint, error) {
	bySource := map[entities.SeriesSource][]entities.SeriesDefinition{}
	sources := []entities.SeriesSource{}

	for _, definition := range definitions {
		if _, seen := bySource[definition.Source]; !seen {
			sources = append(sources, definition.Source)
		}
		bySource[definition.Source] = append(bySource[definition.Source], definition)
	}

	points := map[string][]entities.SeriesPoint{}

	for _, source := range sources {
		reader, ok := s.readers[source]
		if !ok {
			// Error de cableado: hay una variable en el catálogo cuya fuente no tiene lector.
			s.dashboardLogger.Error("[SeriesService] No hay lector para la fuente", "source", source)
			return nil, domain.ErrConsultingSeries
		}

		read, err := reader.Read(ctx, deviceID, bySource[source], startDate, endDate)
		if err != nil {
			s.dashboardLogger.Error("[SeriesService] Error consultando las series",
				"device_id", deviceID, "source", source, "error", err)
			return nil, domain.ErrConsultingSeries
		}

		for key, series := range read {
			points[key] = series
		}
	}

	return points, nil
}
