package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/ports"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestService struct {
	dataProvider          ports.DataProvider
	ingestStateRepository repositories.IngestionStateRepository
	deviceRepository      repositories.DeviceRepository
	unitOfWork            ports.IngestionUnitOfWork
	processingService     *ProcessingService
	ingestionValidator    *IngestionValidator
	logger                *logger.Logger
}

func NewIngestService(
	dataProvider ports.DataProvider,
	ingestStateRepository repositories.IngestionStateRepository,
	deviceRepository repositories.DeviceRepository,
	processingService *ProcessingService,
	unitOfWork ports.IngestionUnitOfWork,
	logger *logger.Logger,
	ingestionValidator *IngestionValidator,
) *IngestService {
	return &IngestService{
		dataProvider:          dataProvider,
		ingestStateRepository: ingestStateRepository,
		deviceRepository:      deviceRepository,
		processingService:     processingService,
		logger:                logger,
		unitOfWork:            unitOfWork,
		ingestionValidator:    ingestionValidator,
	}
}

func (s *IngestService) IngestDataOs(ctx context.Context, measuringPointID int64, start time.Time, end time.Time,
) error {

	now := time.Now()

	// 1. Obtener dispositivo.
	device, err := s.deviceRepository.FindByMeasuringPointID(ctx, measuringPointID)
	if device == nil {
		return appErrors.ErrDeviceNotFound
	}

	// 2. Validar rango solicitado.
	if err := s.ingestionValidator.validateRange(start, end, now); err != nil {
		return err
	}

	// 3. Buscar el último estado de ingesta.
	lastState, err := s.ingestStateRepository.FindLatestByMeasuringPointID(ctx, measuringPointID)
	if err != nil {
		return ingestionErrors.ErrConsultingLastState
	}

	// 4. Determinar desde qué fecha se debe consultar DataOS.
	ingestionStart := start
	if lastState == nil {
		s.logger.Info("no existe un estado de ingesta anterior; se realizará la primera ingesta", "measuring_point_id", measuringPointID, "fecha_inicio", start, "fecha_fin", end)
	}

	// 6. Obtener mediciones desde DataOS.
	measurements, err := s.dataProvider.GetMeasurements(ctx, device, ingestionStart.Add(1*time.Second), end)
	if err != nil {
		return ingestionErrors.ErrGettingIngestedDataInDataOS
	}

	s.logger.Info("mediciones obtenidas desde DataOS", "measuring_point_id", measuringPointID, "cantidad", len(measurements))

	// 7. Procesar mediciones.
	processedMeasurements, err := s.processingService.Process(ctx, measurements)
	if err != nil {
		s.logger.Error("ocurrió un error durante el procesamiento de las mediciones", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("procesar mediciones: %w", err)
	}

	// 8. Si no llegaron datos, no se crea estado.
	if len(processedMeasurements) == 0 {
		return ingestionErrors.ErrRecivingIngestedData
	}

	// 9. Obtener timestamp de la última medición.
	lastIngestedAt := getLatestMeasurementTimestamp(processedMeasurements)

	// 10. Crear el estado de ingesta.
	// Para la primera ingesta:
	//     InitialDate = start solicitado.

	state := &entities.IngestionState{
		MeasuringPointID: uint64(measuringPointID),
		Source:           "dataos",
		ReceivedRecords:  len(processedMeasurements),
		LastIngestedAt:   lastIngestedAt,
	}

	s.logger.Info("se preparó el estado de la ingesta", "measuring_point_id", measuringPointID,
		"received_records", state.ReceivedRecords, "last_ingested_at", state.LastIngestedAt)

	// 11. Persistir mediciones y estado dentro de la misma transacción.
	err = s.unitOfWork.Execute(
		ctx,
		func(
			ctx context.Context,
			measurementRepository repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
			ingestionGapRepository repositories.IngestionGapRepository,
		) error {

			// Guardar mediciones.
			if err := measurementRepository.SaveBatch(
				ctx,
				processedMeasurements,
			); err != nil {
				return fmt.Errorf("guardar mediciones: %w", err)
			}

			s.logger.Info(
				"mediciones guardadas correctamente",
				"measuring_point_id", measuringPointID,
				"cantidad", len(processedMeasurements),
			)

			// Guardar estado.
			if err := ingestionStateRepository.Save(
				ctx,
				state,
			); err != nil {
				return fmt.Errorf("guardar estado de ingesta: %w", err)
			}

			s.logger.Info(
				"estado de ingesta guardado correctamente",
				"measuring_point_id", measuringPointID,
				"received_records", state.ReceivedRecords,
				"last_ingested_at", state.LastIngestedAt,
			)

			// Guardar/actualizar gap de ingesta.
			// Aquí irá la lógica cuando tengamos el gap detectado.
			// Por ahora no se persiste nada.

			_ = ingestionGapRepository

			return nil
		},
	)

	if err != nil {
		s.logger.Error(
			"la transacción de ingesta falló y fue revertida",
			"measuring_point_id", measuringPointID,
			"error", err,
		)

		return fmt.Errorf("persistir ingesta: %w", err)
	}

	s.logger.Info(
		"ingesta completada correctamente",
		"measuring_point_id", measuringPointID,
		"received_records", state.ReceivedRecords,
		"last_ingested_at", state.LastIngestedAt,
	)

	return nil
}

func getLatestMeasurementTimestamp(measurements []entities.RawMeasurement,
) *time.Time {

	if len(measurements) == 0 {
		return nil
	}

	var latest time.Time

	for _, measurement := range measurements {
		if latest.IsZero() || measurement.Timestamp.After(latest) {
			latest = measurement.Timestamp
		}
	}
	return &latest
}
