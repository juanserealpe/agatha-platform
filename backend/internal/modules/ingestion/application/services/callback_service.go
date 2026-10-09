package services

import (
	"context"
	"errors"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/ports"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// CallbackSource identifica en ingestion_logs los lotes recibidos por callback.
const CallbackSource = "dataflow_callback"

// CallbackService recibe los registros que Dataflow Labs empuja por HTTP.
//
// Por ahora NO mapea variables a métricas ni escribe raw_measurements: registra
// en el log qué variables llegaron y persiste el recibo del registro (una fila
// en ingestion_logs con su record_id) para deduplicar los reintentos.
type CallbackService struct {
	deviceRepository repositories.DeviceRepository
	unitOfWork       ports.IngestionUnitOfWork
	validator        *CallbackValidator
	logger           *logger.Logger
}

func NewCallbackService(
	deviceRepository repositories.DeviceRepository,
	unitOfWork ports.IngestionUnitOfWork,
	validator *CallbackValidator,
	logger *logger.Logger,
) *CallbackService {
	return &CallbackService{
		deviceRepository: deviceRepository,
		unitOfWork:       unitOfWork,
		validator:        validator,
		logger:           logger,
	}
}

// Receive valida, registra y persiste un registro. Devuelve duplicate si el
// record_id ya fue procesado; cualquier error de BD es reintentable (5xx).
func (s *CallbackService) Receive(ctx context.Context, request dtos.DataRecordCallback) (dtos.CallbackResponse, error) {
	// 1. Validar el payload.
	record, err := s.validator.Validate(request)
	if err != nil {
		s.logger.Warn("[Callback] payload rechazado", "record_id", request.RecordID, "mp_id", request.MpID, "error", err)
		return dtos.CallbackResponse{}, err
	}

	// 2. Mostrar qué llegó (sin mapear todavía a métricas).
	s.logReceivedRecord(record)

	// 3. Verificar que el punto de medición exista.
	device, err := s.deviceRepository.FindByMeasuringPointID(ctx, record.MeasuringPointID)
	if err != nil {
		s.logger.Error("[Callback] error consultando el punto de medición", "record_id", record.RecordID, "mp_id", record.MeasuringPointID, "error", err)
		return dtos.CallbackResponse{}, ingestionErrors.ErrConsultingDevice
	}
	if device == nil {
		s.logger.Warn("[Callback] punto de medición no registrado", "record_id", record.RecordID, "mp_id", record.MeasuringPointID)
		return dtos.CallbackResponse{}, appErrors.ErrDeviceNotFound
	}

	// 4. Persistir el recibo; el índice único sobre record_id deduplica.
	recordID := record.RecordID
	lastIngestedAt := record.LatestReadingTimestamp()
	state := &entities.IngestionState{
		MeasuringPointID: uint64(record.MeasuringPointID),
		Source:           CallbackSource,
		LastIngestedAt:   &lastIngestedAt,
		ReceivedRecords:  len(record.Readings),
		RecordID:         &recordID,
	}

	var created bool
	err = s.unitOfWork.Execute(
		ctx,
		func(
			ctx context.Context,
			_ repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
			_ repositories.IngestionGapRepository,
		) error {
			// Cuando exista el mapeo de variables → metric_id, las mediciones se
			// guardan acá, en la misma TX, DESPUÉS de reclamar el record_id.
			_, saved, err := ingestionStateRepository.SaveIfAbsent(ctx, state)
			created = saved
			return err
		},
	)
	if err != nil {
		s.logger.Error("[Callback] no se pudo persistir el registro; Dataflow Labs debe reintentar", "record_id", record.RecordID, "mp_id", record.MeasuringPointID, "error", err)
		return dtos.CallbackResponse{}, errors.Join(ingestionErrors.ErrPersistingIngestionState, err)
	}

	response := dtos.CallbackResponse{
		Status:          dtos.CallbackStatusStored,
		RecordID:        record.RecordID,
		ReceivedRecords: len(record.Readings),
	}

	if !created {
		response.Status = dtos.CallbackStatusDuplicate
		s.logger.Info("[Callback] registro duplicado; ya había sido procesado", "record_id", record.RecordID, "mp_id", record.MeasuringPointID)
		return response, nil
	}

	s.logger.Info("[Callback] registro persistido", "record_id", record.RecordID, "mp_id", record.MeasuringPointID, "received_records", response.ReceivedRecords, "last_ingested_at", lastIngestedAt)
	return response, nil
}

func (s *CallbackService) logReceivedRecord(record *entities.CallbackRecord) {
	names := make([]string, 0, len(record.Readings))
	for _, reading := range record.Readings {
		names = append(names, reading.Name)
	}

	s.logger.Info("[Callback] registro recibido",
		"record_id", record.RecordID,
		"mp_id", record.MeasuringPointID,
		"serial_id", record.SerialID,
		"timestamp", record.Timestamp,
		"variables", len(record.Readings),
		"names", names,
	)

	for _, reading := range record.Readings {
		s.logger.Info("[Callback] variable recibida",
			"record_id", record.RecordID,
			"device_id", reading.DeviceID,
			"name", reading.Name,
			"readings", reading.Value,
			"timestamp", reading.RawTimestamp,
		)
	}
}
