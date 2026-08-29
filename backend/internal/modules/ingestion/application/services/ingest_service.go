package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestService struct {
	dataProvider               ports.DataProvider
	deviceRepository           repositories.DeviceRepository
	metricDefinitionRepository repositories.MetricDefinitionRepository
	measurementRepository      repositories.MeasurementRepository

	logger *logger.Logger
}

func NewIngestService(
	dataProvider ports.DataProvider,
	deviceRepository repositories.DeviceRepository,
	metricDefinitionRepository repositories.MetricDefinitionRepository,
	measurementRepository repositories.MeasurementRepository,
	logger *logger.Logger,
) *IngestService {
	return &IngestService{
		dataProvider:               dataProvider,
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
	MeasuringPointID int64,
	start time.Time,
	end time.Time,
) error {

	device, err := s.getActiveDevice(ctx, MeasuringPointID)
	if err != nil {
		return err
	}

	measurements, err := s.dataProvider.GetMeasurements(
		ctx,
		device,
		start,
		end,
	)
	if err != nil {
		return err
	}

	return s.saveMeasurements(ctx, device.MeasuringPointID, measurements)
}

func (s *IngestService) getActiveDevice(
	ctx context.Context,
	MeasuringPointID int64,
) (*entities.Device, error) {

	device, err := s.deviceRepository.FindByMeasuringPointID(ctx, MeasuringPointID)

	if err != nil {
		return nil, fmt.Errorf(
			"find device %q: %w",
			MeasuringPointID,
			err,
		)
	}

	if device == nil {
		return nil, fmt.Errorf(
			"device %q not found",
			MeasuringPointID,
		)
	}

	if !device.Active {
		return nil, fmt.Errorf(
			"device %q is inactive",
			device.MeasuringPointID,
		)
	}

	return device, nil
}

func (s *IngestService) saveMeasurements(
	ctx context.Context,
	deviceID int64,
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
