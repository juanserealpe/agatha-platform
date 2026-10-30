package services

import (
	"context"
	"errors"
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
	ingestionValidator    *IngestionValidator
	eventPublisher        ports.EventPublisher
	now                   func() time.Time
	logger                *logger.Logger
}

func NewIngestService(
	dataProvider ports.DataProvider,
	ingestStateRepository repositories.IngestionStateRepository,
	deviceRepository repositories.DeviceRepository,
	unitOfWork ports.IngestionUnitOfWork,
	logger *logger.Logger,
	ingestionValidator *IngestionValidator,
	eventPublisher ports.EventPublisher,
) *IngestService {
	return &IngestService{
		dataProvider:          dataProvider,
		ingestStateRepository: ingestStateRepository,
		deviceRepository:      deviceRepository,
		logger:                logger,
		unitOfWork:            unitOfWork,
		ingestionValidator:    ingestionValidator,
		eventPublisher:        eventPublisher,
		now:                   time.Now,
	}
}

// IngestDataOs consulta DataOS en (start, end] y persiste las mediciones junto
// con su estado de ingesta en una sola transacción. Publica MeasurementsIngested
// después del commit.
func (s *IngestService) IngestDataOs(ctx context.Context, measuringPointID int64, start time.Time, end time.Time,
) error {
	// 1. Obtener dispositivo.
	device, err := s.deviceRepository.FindByMeasuringPointID(ctx, measuringPointID)
	if err != nil {
		s.logger.Error("error consultando el punto de medición", "mp_id", measuringPointID, "error", err)
		return ingestionErrors.ErrConsultingDevice
	}
	if device == nil {
		s.logger.Error("punto de medición no registrado", "mp_id", measuringPointID)
		return appErrors.ErrDeviceNotFound
	}

	// 2. Validar rango solicitado.
	if err := s.ingestionValidator.validateRange(start, end); err != nil {
		return err
	}

	// 3. Buscar el último estado de ingesta.
	lastState, err := s.ingestStateRepository.FindLatestByMeasuringPointID(ctx, measuringPointID)
	if err != nil {
		s.logger.Error("error consultando el último estado de ingesta", "mp_id", measuringPointID, "error", err)
		return ingestionErrors.ErrConsultingLastState
	}
	if lastState == nil {
		s.logger.Info("primera ingesta del punto de medición", "mp_id", measuringPointID)
	}

	// 4. Obtener mediciones desde DataOS (start es exclusivo).
	measurements, err := s.dataProvider.GetMeasurements(ctx, device, start.Add(time.Second), end)
	if err != nil {
		return ingestionErrors.ErrGettingIngestedDataInDataOS
	}

	// 5. Si no llegaron datos, no se crea estado.
	if len(measurements) == 0 {
		return ingestionErrors.ErrRecivingIngestedData
	}

	// 6. last_ingested_at es el MAX(timestamp) del lote, no el fin del rango pedido.
	state := &entities.IngestionState{
		MeasuringPointID: uint64(measuringPointID),
		Source:           "dataos",
		ReceivedRecords:  len(measurements),
		LastIngestedAt:   getLatestMeasurementTimestamp(measurements),
	}

	// 7. Persistir estado y mediciones en la misma transacción. El estado va
	// primero porque su id es el lote (ingestion_batch_id) de cada medición.
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
		) error {
			savedState, err := ingestionStateRepository.Save(ctx, state)
			if err != nil {
				return errors.Join(ingestionErrors.ErrPersistingIngestionState, err)
			}
			linkToBatch(measurements, savedState.BatchSeq)

			// Si fallan las mediciones, se revierte también el estado.
			savedBatch, err := measurementRepository.SaveBatch(ctx, measurements)
			if err != nil {
				return fmt.Errorf("guardar mediciones: %w", err)
			}

			batchResult = savedBatch
			stateResult = savedState
			return nil
		},
	)
	if err != nil {
		s.logger.Error("la ingesta falló y se revirtió", "mp_id", measuringPointID, "error", err)
		return fmt.Errorf("persistir ingesta: %w", err)
	}

	s.logger.Info("ingesta DataOS guardada", "mp_id", measuringPointID, "recibidas", state.ReceivedRecords, "insertadas", batchResult.Inserted, "hasta", state.LastIngestedAt)

	// 8. Publicar el evento post-commit (nunca hace fallar la ingesta).
	s.publishMeasurementsIngested(ctx, measuringPointID, state.Source, measurements, batchResult, stateResult)

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
		s.logger.Warn("no se pudo publicar el evento; la ingesta ya está guardada", "mp_id", measuringPointID, "batch_seq", event.BatchSeq, "error", err)
	}
}

// linkToBatch asigna a cada medición el lote (id de su fila en
// ingestion_state_models) que la trajo.
func linkToBatch(measurements []entities.RawMeasurement, batchSeq int64) {
	for i := range measurements {
		measurements[i].IngestionBatchID = &batchSeq
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
