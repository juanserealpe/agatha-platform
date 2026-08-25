package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/mapping"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestService struct {
	dataosClient    *dataos.Client
	mapper          *mapping.Mapper
	seriesExtractor *mapping.SeriesExtractor

	deviceRepository           repositories.DeviceRepository
	metricDefinitionRepository repositories.MetricDefinitionRepository
	measurementRepository      repositories.MeasurementRepository

	logger *logger.Logger
}

func NewIngestService(
	dataosClient *dataos.Client,
	mapper *mapping.Mapper,
	seriesExtractor *mapping.SeriesExtractor,
	deviceRepository repositories.DeviceRepository,
	metricDefinitionRepository repositories.MetricDefinitionRepository,
	measurementRepository repositories.MeasurementRepository,
	logger *logger.Logger,
) *IngestService {
	return &IngestService{
		dataosClient:               dataosClient,
		mapper:                     mapper,
		seriesExtractor:            seriesExtractor,
		deviceRepository:           deviceRepository,
		metricDefinitionRepository: metricDefinitionRepository,
		measurementRepository:      measurementRepository,
		logger:                     logger,
	}
}

// Ingest orquesta el proceso completo de ingesta de mediciones para un dispositivo.
//
// Recibe:
//   - ctx: contexto de ejecución.
//   - deviceID: identificador del dispositivo.
//   - start: inicio del período de consulta.
//   - end: fin del período de consulta.
//
// Utiliza helpers para:
//   - validar el dispositivo: getActiveDevice
//   - autenticar con DataOS: authenticate
//   - consultar datos: fetchDataOSData
//   - extraer series: seriesExtractor.Extract
//   - mapear mediciones: mapSeries
//   - guardar mediciones: saveMeasurements
//
// Retorna error si alguna etapa del proceso falla.
func (s *IngestService) Ingest(
	ctx context.Context,
	deviceID string,
	start time.Time,
	end time.Time,
) error {

	device, err := s.getActiveDevice(ctx, deviceID)
	if err != nil {
		return err
	}

	if err := s.authenticate(ctx); err != nil {
		return err
	}

	response, err := s.fetchDataOSData(
		ctx,
		device,
		start,
		end,
	)
	if err != nil {
		return err
	}

	series, err := s.seriesExtractor.Extract(response)
	if err != nil {
		return fmt.Errorf("extract DataOS series: %w", err)
	}

	measurements, err := s.mapSeries(
		ctx,
		device,
		series,
	)
	if err != nil {
		return err
	}

	return s.saveMeasurements(ctx, device.ID, measurements)
}

func (s *IngestService) getActiveDevice(
	ctx context.Context,
	deviceID string,
) (*entities.Device, error) {

	device, err := s.deviceRepository.FindByID(ctx, deviceID)

	if err != nil {
		return nil, fmt.Errorf(
			"find device %q: %w",
			deviceID,
			err,
		)
	}

	if device == nil {
		return nil, fmt.Errorf(
			"device %q not found",
			deviceID,
		)
	}

	if !device.Active {
		return nil, fmt.Errorf(
			"device %q is inactive",
			device.ID,
		)
	}

	return device, nil
}
func (s *IngestService) authenticate(
	ctx context.Context,
) error {

	if err := s.dataosClient.Login(ctx); err != nil {
		return fmt.Errorf(
			"authenticate with DataOS: %w",
			err,
		)
	}

	return nil
}

func (s *IngestService) fetchDataOSData(
	ctx context.Context,
	device *entities.Device,
	start time.Time,
	end time.Time,
) (*dashboard.RenderResponse, error) {

	request := dashboard.RenderRequest{
		Period: dashboard.Period{
			Type:  "absolute",
			Start: start.Format("2006-01-02T15:04:05"),
			End:   end.Format("2006-01-02T15:04:05"),
		},
		Timezone:         "America/Bogota",
		MeasuringPointID: device.MeasuringPointID,
	}

	response, err := s.dataosClient.Render(ctx, request)

	if err != nil {
		return nil, fmt.Errorf(
			"fetch DataOS measurements: %w",
			err,
		)
	}

	return response, nil
}

func (s *IngestService) mapSeries(
	ctx context.Context,
	device *entities.Device,
	series []dashboard.Series,
) ([]entities.RawMeasurement, error) {

	var measurements []entities.RawMeasurement

	for _, serie := range series {

		metricKey, ok := mapping.MetricKeyFromDataOS(
			serie.Name,
		)

		if !ok {
			s.logger.Warn(
				"métrica de DataOS no reconocida",
				"metric", serie.Name,
			)

			continue
		}

		metricDefinition, err :=
			s.metricDefinitionRepository.FindByKey(
				ctx,
				metricKey,
			)

		if err != nil {
			return nil, fmt.Errorf(
				"find metric definition %q: %w",
				metricKey,
				err,
			)
		}

		if metricDefinition == nil {
			s.logger.Warn(
				"definición de métrica no encontrada",
				"metric_key", metricKey,
			)

			continue
		}

		mapped, err := s.mapper.MapSeries(
			device,
			serie,
			metricKey,
			metricDefinition.ID,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"map metric %q: %w",
				serie.Name,
				err,
			)
		}

		measurements = append(
			measurements,
			mapped...,
		)
	}

	return measurements, nil
}

func (s *IngestService) saveMeasurements(
	ctx context.Context,
	deviceID string,
	measurements []entities.RawMeasurement,
) error {

	s.logger.Info(
		"guardando mediciones en BD",
		"device_id", deviceID,
		"measurements", len(measurements),
	)

	if err := s.measurementRepository.SaveBatch(
		ctx,
		measurements,
	); err != nil {

		return fmt.Errorf(
			"save measurements: %w",
			err,
		)
	}

	return nil
}
