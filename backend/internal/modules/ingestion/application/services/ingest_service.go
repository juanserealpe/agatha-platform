package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
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
	eventPublisher        ports.EventPublisher
	now                   func() time.Time
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
	eventPublisher ports.EventPublisher,
) *IngestService {
	return &IngestService{
		dataProvider:          dataProvider,
		ingestStateRepository: ingestStateRepository,
		deviceRepository:      deviceRepository,
		processingService:     processingService,
		logger:                logger,
		unitOfWork:            unitOfWork,
		ingestionValidator:    ingestionValidator,
		eventPublisher:        eventPublisher,
		now:                   time.Now,
	}
}

func (s *IngestService) IngestDataOs(ctx context.Context, measuringPointID int64, start time.Time, end time.Time,
) error {

	now := s.now()

	// 1. Obtener dispositivo.
	device, err := s.deviceRepository.FindByMeasuringPointID(ctx, measuringPointID)
	if device == nil {
		s.logger.Error("[IngestDataOs] no se encontró un dispositivo asociado al punto de medición", "measuring_point_id", measuringPointID)
		return appErrors.ErrDeviceNotFound
	}

	// 2. Validar rango solicitado.
	if err := s.ingestionValidator.validateRange(start, end, now); err != nil {
		s.logger.Error("[IngestDataOs] rango de fechas inválido", "measuring_point_id", measuringPointID, "start", start, "end", end, "now", now, "error", err)
		return err
	}

	// 3. Buscar el último estado de ingesta.
	lastState, err := s.ingestStateRepository.FindLatestByMeasuringPointID(ctx, measuringPointID)
	if err != nil {
		s.logger.Error("[IngestDataOs] error consultando el último estado de ingesta", "measuring_point_id", measuringPointID, "error", err)
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

	// 9. Obtener timestamp de la última medición: last_ingested_at es el
	// MAX(timestamp) de las muestras del lote, no el fin del rango pedido.
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
	// Los resultados se capturan para publicar el evento DESPUÉS del commit.
	var (
		batchResult entities.SaveBatchResult
		stateResult entities.SaveStateResult
	)

	err = s.unitOfWork.Execute(
		ctx,
		func(
			ctx context.Context,
			measurementRepository repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
			ingestionGapRepository repositories.IngestionGapRepository,
		) error {

			// Guardar mediciones.
			savedBatch, err := measurementRepository.SaveBatch(
				ctx,
				processedMeasurements,
			)
			if err != nil {
				return fmt.Errorf("guardar mediciones: %w", err)
			}

			s.logger.Info(
				"mediciones guardadas correctamente",
				"measuring_point_id", measuringPointID,
				"cantidad", len(processedMeasurements),
			)

			// Guardar estado (misma TX: si falla, se revierten también las mediciones).
			savedState, err := ingestionStateRepository.Save(
				ctx,
				state,
			)
			if err != nil {
				s.logger.Error(
					"no se pudo guardar el estado de ingesta; se revierte el lote",
					"measuring_point_id", measuringPointID,
					"error", err,
				)
				return ingestionErrors.ErrPersistingIngestionState
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

			batchResult = savedBatch
			stateResult = savedState

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

	// 12. Publicar el evento post-commit (nunca hace fallar la ingesta).
	s.publishMeasurementsIngested(ctx, measuringPointID, state.Source, processedMeasurements, batchResult, stateResult)

	return nil
}

// publishMeasurementsIngested publica MeasurementsIngested si se insertó al menos
// una medición. Debe llamarse solo tras un commit exitoso. Un publisher nil
// equivale a no-op; un error del publisher se registra como warn y se ignora.
func (s *IngestService) publishMeasurementsIngested(
	ctx context.Context,
	measuringPointID int64,
	source string,
	measurements []entities.RawMeasurement,
	batchResult entities.SaveBatchResult,
	stateResult entities.SaveStateResult,
) {
	if s.eventPublisher == nil || batchResult.Inserted <= 0 || len(measurements) == 0 {
		return
	}

	from, to := getMeasurementTimeRange(measurements)
	event := contract.MeasurementsIngested{
		MeasuringPointID: measuringPointID,
		Source:           source,
		From:             from.UTC(),
		To:               to.UTC(),
		Watermark:        stateResult.LastIngestedAt.UTC(),
		BatchSeq:         stateResult.BatchSeq,
		InsertedRecords:  batchResult.Inserted,
		OccurredAt:       s.now().UTC(),
		SchemaVersion:    contract.SchemaVersion,
	}

	if err := s.eventPublisher.PublishMeasurementsIngested(ctx, event); err != nil {
		s.logger.Warn(
			"no se pudo publicar el evento de mediciones ingeridas; la ingesta ya fue confirmada",
			"measuring_point_id", measuringPointID,
			"batch_seq", event.BatchSeq,
			"error", err,
		)
	}
}

// getMeasurementTimeRange devuelve el timestamp mínimo y máximo del lote.
// Requiere al menos una medición.
func getMeasurementTimeRange(measurements []entities.RawMeasurement) (time.Time, time.Time) {
	from, to := measurements[0].Timestamp, measurements[0].Timestamp
	for _, measurement := range measurements[1:] {
		if measurement.Timestamp.Before(from) {
			from = measurement.Timestamp
		}
		if measurement.Timestamp.After(to) {
			to = measurement.Timestamp
		}
	}
	return from, to
}

// getLatestMeasurementTimestamp devuelve el MAX(timestamp) del lote (nil si está vacío).
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
