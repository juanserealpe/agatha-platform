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

// IngestService orquesta el proceso de ingesta de datos.
type IngestService struct {
	dataProvider          ports.DataProvider
	ingestStateRepository repositories.IngestionStateRepository
	unitOfWork            ports.IngestionUnitOfWork
	processingService     *ProcessingService
	ingestionValidator    *IngestionValidator
	logger                *logger.Logger
}

func NewIngestService(
	dataProvider ports.DataProvider,
	ingestStateRepository repositories.IngestionStateRepository,
	processingService *ProcessingService,
	unitOfWork ports.IngestionUnitOfWork,
	logger *logger.Logger,
	ingestionValidator *IngestionValidator,
) *IngestService {
	return &IngestService{
		dataProvider:          dataProvider,
		ingestStateRepository: ingestStateRepository,
		processingService:     processingService,
		logger:                logger,
		unitOfWork:            unitOfWork,
		ingestionValidator:    ingestionValidator,
	}
}

func (s *IngestService) IngestDataOs(ctx context.Context, measuringPointID int64, start time.Time, end time.Time) error {

	now := time.Now()
	fmt.Println("Tiempo actual : ", now)
	fmt.Println("Tiempo inicio parámetro: ", start, "Tiempo fin parámetro: ", end)

	if err := s.ingestionValidator.validateRange(start, end, now); err != nil {
		return err
	}

	device, err := s.ingestionValidator.getActiveDevice(ctx, measuringPointID)
	if err != nil {
		s.logger.Error("no fue posible obtener el dispositivo", "measuring_point_id", measuringPointID, "error", err)
		return err
	}

	lastState, err := s.ingestStateRepository.FindLatestByMeasuringPointID(ctx, measuringPointID)
	if err != nil {
		s.logger.Error("no fue posible consultar el último estado de ingesta", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("consultar último estado de ingesta: %w", err)
	}

	if lastState != nil {
		if err := s.ingestionValidator.validateIngestionRangeHistorical(lastState, start, end, now); err != nil {
			s.logger.Error("el rango solicitado entra en conflicto con una ingesta anterior", "measuring_point_id", measuringPointID, "error", err)
			return err
		}
	} else {
		s.logger.Info("no existe un estado de ingesta anterior; se verificará la disponibilidad de datos", "measuring_point_id", measuringPointID)
	}
	startAt := lastState.LastIngestedAt.Add(time.Second)
	measurements, err := s.dataProvider.GetMeasurements(ctx, device, startAt, end)
	if err != nil {
		s.logger.Error("ocurrió un error al obtener mediciones desde DataOS", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("obtener mediciones desde DataOS: %w", err)
	}

	s.logger.Info("mediciones obtenidas desde DataOS", "measuring_point_id", measuringPointID, "cantidad", len(measurements))

	processedMeasurements, err := s.processingService.Process(ctx, measurements)
	if err != nil {
		s.logger.Error("ocurrió un error durante el procesamiento de las mediciones", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("procesar mediciones: %w", err)
	}

	if len(processedMeasurements) == 0 {
		s.logger.Info("no se recibieron mediciones; no se guardará el estado de ingesta", "measuring_point_id", measuringPointID, "fecha_inicio", start, "fecha_fin", end)
		return nil
	}

	state := &entities.IngestionState{
		MeasuringPointID: uint64(measuringPointID),
		Source:           "dataos",
		InitialDate:      start,
		ReceivedRecords:  len(processedMeasurements),
		LastIngestedAt:   getLatestMeasurementTimestamp(processedMeasurements),
	}

	s.logger.Info("se preparó el estado de la ingesta", "measuring_point_id", measuringPointID, "received_records", state.ReceivedRecords, "last_ingested_at", state.LastIngestedAt)

	err = s.unitOfWork.Execute(ctx, func(
		ctx context.Context,
		measurementRepository repositories.MeasurementRepository,
		ingestionStateRepository repositories.IngestionStateRepository,
	) error {
		if err := measurementRepository.SaveBatch(ctx, processedMeasurements); err != nil {
			return fmt.Errorf("guardar mediciones: %w", err)
		}

		s.logger.Info("mediciones guardadas correctamente", "measuring_point_id", measuringPointID, "cantidad", len(processedMeasurements))

		if err := ingestionStateRepository.Save(ctx, state); err != nil {
			return fmt.Errorf("guardar estado de ingesta: %w", err)
		}

		s.logger.Info("estado de ingesta guardado correctamente", "measuring_point_id", measuringPointID, "received_records", state.ReceivedRecords, "last_ingested_at", state.LastIngestedAt)

		return nil
	})

	if err != nil {
		s.logger.Error("la transacción de ingesta falló y fue revertida", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("persistir ingesta: %w", err)
	}

	s.logger.Info("ingesta completada correctamente", "measuring_point_id", measuringPointID, "received_records", state.ReceivedRecords)

	return nil
}

func getLatestMeasurementTimestamp(measurements []entities.RawMeasurement) *time.Time {
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
