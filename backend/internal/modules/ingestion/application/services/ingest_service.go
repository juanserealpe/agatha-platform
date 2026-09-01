package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// IngestService orquesta el proceso de ingesta de datos.
type IngestService struct {
	dataProvider          ports.DataProvider
	deviceRepository      repositories.DeviceRepository
	ingestStateRepository repositories.IngestionStateRepository
	unitOfWork            ports.IngestionUnitOfWork
	processingService     *ProcessingService

	logger *logger.Logger
}

func NewIngestService(
	dataProvider ports.DataProvider,
	deviceRepository repositories.DeviceRepository,
	ingestStateRepository repositories.IngestionStateRepository,
	processingService *ProcessingService,
	unitOfWork ports.IngestionUnitOfWork,
	logger *logger.Logger,
) *IngestService {
	return &IngestService{
		dataProvider:          dataProvider,
		deviceRepository:      deviceRepository,
		ingestStateRepository: ingestStateRepository,
		processingService:     processingService,
		logger:                logger,
		unitOfWork:            unitOfWork,
	}
}

func (s *IngestService) IngestDataOs(
	ctx context.Context,
	measuringPointID int64,
	start time.Time,
	end time.Time,
) error {

	// ---------------------------------------------------------
	// 1. Validar rango básico
	// ---------------------------------------------------------

	now := time.Now()

	if err := s.validateIngestionRange(
		nil,
		start,
		end,
		now,
	); err != nil {
		return err
	}

	// ---------------------------------------------------------
	// 2. Buscar dispositivo
	// ---------------------------------------------------------

	device, err := s.getActiveDevice(
		ctx,
		measuringPointID,
	)
	if err != nil {

		s.logger.Error(
			"no fue posible obtener el dispositivo",
			"measuring_point_id", measuringPointID,
			"error", err,
		)

		return err
	}

	// ---------------------------------------------------------
	// 3. Buscar último estado de ingesta
	// ---------------------------------------------------------

	lastState, err := s.ingestStateRepository.FindLatestByMeasuringPointID(
		ctx,
		measuringPointID,
	)
	if err != nil {

		s.logger.Error(
			"no fue posible consultar el último estado de ingesta",
			"measuring_point_id", measuringPointID,
			"error", err,
		)

		return fmt.Errorf(
			"consultar último estado de ingesta: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// 4. Validar rango contra ingestas anteriores
	// ---------------------------------------------------------

	if lastState != nil {

		if err := s.validateIngestionRange(
			lastState,
			start,
			end,
			now,
		); err != nil {

			s.logger.Error(
				"el rango solicitado entra en conflicto con una ingesta anterior",
				"measuring_point_id", measuringPointID,
				"error", err,
			)

			return err
		}

	} else {

		s.logger.Info(
			"no existe un estado de ingesta anterior; se realizará la primera ingesta",
			"measuring_point_id", measuringPointID,
		)
	}

	// ---------------------------------------------------------
	// 5. Crear estado de la nueva ejecución
	// ---------------------------------------------------------

	state := &entities.IngestionState{
		MeasuringPointID: uint64(measuringPointID),
		Source:           "dataos",
		InitialDate:      start,
		LastIngestedAt:   nil,
		ReceivedRecords:  0,
	}

	// ---------------------------------------------------------
	// 6. Consultar DataOS
	// ---------------------------------------------------------

	measurements, err := s.dataProvider.GetMeasurements(
		ctx,
		device,
		start,
		end,
	)
	if err != nil {

		s.logger.Error(
			"ocurrió un error al obtener mediciones desde DataOS",
			"measuring_point_id", measuringPointID,
			"error", err,
		)

		return fmt.Errorf(
			"obtener mediciones desde DataOS: %w",
			err,
		)
	}

	s.logger.Info(
		"mediciones obtenidas desde DataOS",
		"measuring_point_id", measuringPointID,
		"cantidad", len(measurements),
	)

	// ---------------------------------------------------------
	// 7. Procesar / normalizar
	// ---------------------------------------------------------

	processedMeasurements, err := s.processingService.Process(
		ctx,
		measurements,
	)
	if err != nil {

		s.logger.Error(
			"ocurrió un error durante el procesamiento de las mediciones",
			"measuring_point_id", measuringPointID,
			"error", err,
		)

		return fmt.Errorf(
			"procesar mediciones: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// 8. Actualizar estado en memoria
	// ---------------------------------------------------------

	state.ReceivedRecords = len(processedMeasurements)

	latest := getLatestMeasurementTimestamp(
		processedMeasurements,
	)

	if latest != nil {

		state.LastIngestedAt = latest

		s.logger.Info(
			"se identificó la última medición recibida",
			"measuring_point_id", measuringPointID,
			"last_ingested_at", *latest,
		)

	} else {

		s.logger.Info(
			"DataOS no devolvió mediciones para el rango solicitado",
			"measuring_point_id", measuringPointID,
			"fecha_inicio", start,
			"fecha_fin", end,
		)
	}

	// ---------------------------------------------------------
	// 9. Persistencia transaccional
	// ---------------------------------------------------------

	err = s.unitOfWork.Execute(
		ctx,
		func(
			ctx context.Context,
			measurementRepository repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
		) error {

			// ---------------------------------------------
			// Guardar mediciones
			// ---------------------------------------------

			if len(processedMeasurements) > 0 {

				if err := measurementRepository.SaveBatch(
					ctx,
					processedMeasurements,
				); err != nil {

					return fmt.Errorf(
						"guardar mediciones: %w",
						err,
					)
				}

				s.logger.Info(
					"mediciones guardadas correctamente",
					"measuring_point_id", measuringPointID,
					"cantidad", len(processedMeasurements),
				)

			} else {

				s.logger.Info(
					"no hay mediciones para guardar",
					"measuring_point_id", measuringPointID,
				)
			}

			// ---------------------------------------------
			// Guardar estado histórico
			// ---------------------------------------------

			if err := ingestionStateRepository.Save(
				ctx,
				state,
			); err != nil {

				return fmt.Errorf(
					"guardar estado de ingesta: %w",
					err,
				)
			}

			s.logger.Info(
				"estado de ingesta guardado correctamente",
				"measuring_point_id", measuringPointID,
				"received_records", state.ReceivedRecords,
				"last_ingested_at", state.LastIngestedAt,
			)

			return nil
		},
	)

	if err != nil {

		s.logger.Error(
			"la transacción de ingesta falló y fue revertida",
			"measuring_point_id", measuringPointID,
			"error", err,
		)

		return fmt.Errorf(
			"persistir ingesta: %w",
			err,
		)
	}

	s.logger.Info(
		"ingesta completada correctamente",
		"measuring_point_id", measuringPointID,
		"received_records", state.ReceivedRecords,
	)

	return nil
}

func (s *IngestService) getActiveDevice(
	ctx context.Context,
	measuringPointID int64,
) (*entities.Device, error) {

	device, err := s.deviceRepository.FindByMeasuringPointID(
		ctx,
		measuringPointID,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"find device %q: %w",
			measuringPointID,
			err,
		)
	}

	if device == nil {
		return nil, fmt.Errorf(
			"%w: dispositivo %d",
			appErrors.ErrNotFound,
			measuringPointID,
		)
	}

	if !device.Active {
		return nil, fmt.Errorf(
			"%w: dispositivo %d",
			appErrors.ErrDeviceInactive,
			device.MeasuringPointID,
		)
	}

	return device, nil
}

func (s *IngestService) validateIngestionRange(
	lastState *entities.IngestionState,
	start time.Time,
	end time.Time,
	now time.Time,
) error {

	if start.IsZero() || end.IsZero() {
		return appErrors.ErrInvalidRange
	}

	if !start.Before(end) {
		return fmt.Errorf(
			"%w: la fecha de inicio debe ser anterior a la fecha final",
			appErrors.ErrInvalidRange,
		)
	}

	if end.After(now) {
		return fmt.Errorf(
			"%w: fecha final %s",
			appErrors.ErrFutureDate,
			end,
		)
	}

	if lastState == nil {
		return nil
	}

	if start.Before(lastState.InitialDate) {
		return fmt.Errorf(
			"%w: inicio %s, fecha inicial %s",
			appErrors.ErrBeforeInitialDate,
			start,
			lastState.InitialDate,
		)
	}

	if lastState.LastIngestedAt != nil &&
		!start.After(*lastState.LastIngestedAt) {

		return fmt.Errorf(
			"%w: inicio %s, última fecha ingerida %s",
			appErrors.ErrAlreadyIngested,
			start,
			*lastState.LastIngestedAt,
		)
	}

	return nil
}

func getLatestMeasurementTimestamp(
	measurements []entities.RawMeasurement,
) *time.Time {

	if len(measurements) == 0 {
		return nil
	}

	var latest time.Time

	for _, measurement := range measurements {
		if latest.IsZero() ||
			measurement.Timestamp.After(latest) {

			latest = measurement.Timestamp
		}
	}

	return &latest
}
