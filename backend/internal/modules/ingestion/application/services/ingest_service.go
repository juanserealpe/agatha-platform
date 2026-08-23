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

func (s *IngestService) Ingest(
	ctx context.Context,
	device *entities.Device,
	start time.Time,
	end time.Time,
) error {

	s.logger.Info(
		"iniciando ingesta de datos",
		"device_id", device.ID,
		"measuring_point_id", device.MeasuringPointID,
		"start", start,
		"end", end,
	)

	// ---------------------------------------------------------
	// 1. Autenticación
	// ---------------------------------------------------------

	s.logger.Info(
		"autenticando con DataOS",
		"device_id", device.ID,
	)

	if err := s.dataosClient.Login(ctx); err != nil {

		s.logger.Error(
			"falló la autenticación con DataOS",
			"device_id", device.ID,
			"error", err,
		)

		return fmt.Errorf(
			"authenticate with DataOS: %w",
			err,
		)
	}

	s.logger.Info(
		"autenticación con DataOS exitosa",
		"device_id", device.ID,
	)

	// ---------------------------------------------------------
	// 2. Construir solicitud
	// ---------------------------------------------------------

	request := dashboard.RenderRequest{
		Period: dashboard.Period{
			Type:  "absolute",
			Start: start.Format("2006-01-02T15:04:05"),
			End:   end.Format("2006-01-02T15:04:05"),
		},
		Timezone:         "America/Bogota",
		MeasuringPointID: device.MeasuringPointID,
	}

	s.logger.Info(
		"solicitando mediciones a DataOS",
		"device_id", device.ID,
		"measuring_point_id", device.MeasuringPointID,
		"start", request.Period.Start,
		"end", request.Period.End,
	)

	// ---------------------------------------------------------
	// 3. Solicitar a DataOS
	// ---------------------------------------------------------

	response, err := s.dataosClient.Render(ctx, request)
	if err != nil {

		s.logger.Error(
			"falló la solicitud de mediciones a DataOS",
			"device_id", device.ID,
			"measuring_point_id", device.MeasuringPointID,
			"error", err,
		)

		return fmt.Errorf(
			"fetch DataOS measurements: %w",
			err,
		)
	}

	s.logger.Info(
		"mediciones recibidas de DataOS",
		"device_id", device.ID,
		"components", len(response.Components),
	)

	// ---------------------------------------------------------
	// 4. Extraer series
	// ---------------------------------------------------------

	series, err := s.seriesExtractor.Extract(response)
	if err != nil {

		s.logger.Error(
			"falló la extracción de series de DataOS",
			"device_id", device.ID,
			"error", err,
		)

		return fmt.Errorf(
			"extract DataOS series: %w",
			err,
		)
	}

	s.logger.Info(
		"series extraídas de DataOS",
		"device_id", device.ID,
		"series_count", len(series),
	)

	// ---------------------------------------------------------
	// 5. Mapear series
	// ---------------------------------------------------------

	for _, serie := range series {

		s.logger.Info(
			"mapeando métrica de DataOS",
			"device_id", device.ID,
			"metric", serie.Name,
			"unit", serie.Unit,
			"data_points", len(serie.DataPoints),
		)

		mapped, err := s.mapper.MapSeries(
			device,
			serie,
		)

		if err != nil {

			s.logger.Error(
				"falló el mapeo de la métrica de DataOS",
				"device_id", device.ID,
				"metric", serie.Name,
				"error", err,
			)

			continue
		}

		s.logger.Info(
			"métrica de DataOS mapeada exitosamente",
			"device_id", device.ID,
			"metric", mapped.Metric.Name,
			"key", mapped.Metric.Key,
			"category", mapped.Metric.Category,
			"unit", mapped.Metric.Unit,
			"measurements", len(mapped.Measurements),
		)

		// ---------------------------------------------------------
		// Vista previa de máximo 3 mediciones
		// ---------------------------------------------------------

		previewLimit := 3

		if len(mapped.Measurements) < previewLimit {
			previewLimit = len(mapped.Measurements)
		}

		for i := 0; i < previewLimit; i++ {

			measurement := mapped.Measurements[i]

			s.logger.Info(
				"medición capturada",
				"device_id", measurement.DeviceID,
				"metric", mapped.Metric.Name,
				"metric_id", measurement.MetricID,
				"timestamp", measurement.Timestamp.Format(time.RFC3339),
				"value", measurement.Value,
				"source", measurement.Source,
				"external_key", measurement.ExternalKey,
			)
		}

		// ---------------------------------------------------------
		// Persistencia
		// ---------------------------------------------------------

		if err := s.metricDefinitionRepository.Save(
			ctx,
			mapped.Metric,
		); err != nil {

			s.logger.Error(
				"falló la persistencia de la métrica",
				"device_id", device.ID,
				"metric", mapped.Metric.Name,
				"metric_id", mapped.Metric.ID,
				"error", err,
			)

			return fmt.Errorf(
				"persist metric definition: %w",
				err,
			)
		}

		if err := s.measurementRepository.SaveBatch(
			ctx,
			mapped.Measurements,
		); err != nil {

			s.logger.Error(
				"falló la persistencia de mediciones",
				"device_id", device.ID,
				"metric", mapped.Metric.Name,
				"metric_id", mapped.Metric.ID,
				"measurements", len(mapped.Measurements),
				"error", err,
			)

			return fmt.Errorf(
				"persist measurements: %w",
				err,
			)
		}

		s.logger.Info(
			"datos persistidos exitosamente",
			"device_id", device.ID,
			"metric", mapped.Metric.Name,
			"metric_id", mapped.Metric.ID,
			"measurements", len(mapped.Measurements),
		)
	}

	s.logger.Info(
		"ingesta de datos completada exitosamente",
		"device_id", device.ID,
		"series_count", len(series),
	)

	return nil
}
