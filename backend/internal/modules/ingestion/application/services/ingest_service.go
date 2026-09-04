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

func (s *IngestService) IngestDataOs(ctx context.Context, measuringPointID int64, start time.Time, end time.Time,
) error {

	now := time.Now()

	// 1. Obtener dispositivo activo.
	device, err := s.ingestionValidator.getActiveDevice(ctx, measuringPointID)
	if err != nil {
		s.logger.Error(
			"no fue posible obtener el dispositivo",
			"measuring_point_id", measuringPointID,
			"error", err,
		)
		return err
	}

	// 2. Validar rango solicitado.
	if err := s.ingestionValidator.validateRange(start, end, now); err != nil {
		return err
	}

	// 3. Buscar el último estado de ingesta.
	lastState, err := s.ingestStateRepository.FindLatestByMeasuringPointID(
		ctx,
		measuringPointID,
	)
	if err != nil {
		s.logger.Error("no fue posible consultar el último estado de ingesta", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("consultar último estado de ingesta: %w", err)
	}

	// 4. Determinar desde qué fecha se debe consultar DataOS.
	//
	// Primera ingesta:
	//     no existe lastState -> se utiliza start.
	//
	// Ingestas posteriores:
	//     existe lastState -> se continúa desde LastIngestedAt.
	ingestionStart := start

	if lastState == nil {
		s.logger.Info("no existe un estado de ingesta anterior; se realizará la primera ingesta", "measuring_point_id", measuringPointID, "fecha_inicio", start, "fecha_fin", end)
	} else {
		if err := s.ingestionValidator.validateIngestionRangeHistorical(lastState, start, end, now); err != nil {
			s.logger.Error("el rango solicitado entra en conflicto con una ingesta anterior", "measuring_point_id", measuringPointID, "error", err)
			return err
		}

		// Si existe estado anterior pero todavía no tiene
		// LastIngestedAt, utilizamos el start solicitado.
		if lastState.LastIngestedAt != nil {
			ingestionStart = *lastState.LastIngestedAt

			s.logger.Info("se continuará la ingesta desde la última medición registrada", "measuring_point_id", measuringPointID, "last_ingested_at", lastState.LastIngestedAt, "fecha_fin", end)
		} else {
			s.logger.Info("existe un estado de ingesta sin última medición; se utilizará el inicio solicitado", "measuring_point_id", measuringPointID, "fecha_inicio", start, "fecha_fin", end)
		}
	}

	// 5. Normalizar las fechas antes de enviarlas al proveedor.
	s.logger.Info("consultando mediciones", "measuring_point_id", measuringPointID, "fecha_inicio", ingestionStart, "fecha_fin", end)

	// 6. Obtener mediciones desde DataOS.
	measurements, err := s.dataProvider.GetMeasurements(ctx, device, ingestionStart.Add(1*time.Second), end)
	if err != nil {
		s.logger.Error("ocurrió un error al obtener mediciones desde DataOS", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("obtener mediciones desde DataOS: %w", err)
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
		s.logger.Info("no se recibieron mediciones; no se guardará el estado de ingesta", "measuring_point_id", measuringPointID, "fecha_inicio", ingestionStart, "fecha_fin", end)
		return nil
	}

	// 9. Obtener timestamp de la última medición.
	lastIngestedAt := getLatestMeasurementTimestamp(processedMeasurements)

	// 10. Crear el estado de ingesta.
	//
	// Para la primera ingesta:
	//     InitialDate = start solicitado.
	//
	// Para ingestas posteriores:
	//     conservamos el InitialDate original.
	initialDate := start

	if lastState != nil {
		initialDate = lastState.InitialDate
	}

	state := &entities.IngestionState{
		MeasuringPointID: uint64(measuringPointID),
		Source:           "dataos",
		InitialDate:      initialDate,
		ReceivedRecords:  len(processedMeasurements),
		LastIngestedAt:   lastIngestedAt,
	}

	s.logger.Info("se preparó el estado de la ingesta", "measuring_point_id", measuringPointID, "initial_date", state.InitialDate,
		"received_records", state.ReceivedRecords, "last_ingested_at", state.LastIngestedAt)

	// 11. Persistir mediciones y estado dentro de la misma transacción.
	err = s.unitOfWork.Execute(
		ctx,
		func(
			ctx context.Context,
			measurementRepository repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
		) error {

			// Guardar mediciones.
			if err := measurementRepository.SaveBatch(
				ctx,
				processedMeasurements,
			); err != nil {
				return fmt.Errorf("guardar mediciones: %w", err)
			}

			s.logger.Info("mediciones guardadas correctamente", "measuring_point_id", measuringPointID, "cantidad", len(processedMeasurements))

			// Guardar estado.
			if err := ingestionStateRepository.Save(ctx, state); err != nil {
				return fmt.Errorf("guardar estado de ingesta: %w", err)
			}

			s.logger.Info("estado de ingesta guardado correctamente", "measuring_point_id", measuringPointID, "initial_date", state.InitialDate, "received_records",
				state.ReceivedRecords, "last_ingested_at", state.LastIngestedAt)

			return nil
		},
	)

	if err != nil {
		s.logger.Error("la transacción de ingesta falló y fue revertida", "measuring_point_id", measuringPointID, "error", err)
		return fmt.Errorf("persistir ingesta: %w", err)
	}

	s.logger.Info("ingesta completada correctamente", "measuring_point_id", measuringPointID, "received_records", state.ReceivedRecords, "last_ingested_at", state.LastIngestedAt)
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
