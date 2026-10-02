package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// LatestIngestionService lee de la BD la última fecha ingerida de un punto de
// medición y las variables registradas en ella.
type LatestIngestionService struct {
	measurementRepository repositories.MeasurementRepository
	processingLogger      *logger.Logger
}

func NewLatestIngestionService(
	measurementRepository repositories.MeasurementRepository,
	processingLogger *logger.Logger,
) *LatestIngestionService {
	return &LatestIngestionService{
		measurementRepository: measurementRepository,
		processingLogger:      processingLogger,
	}
}

// GetLatestIngestion devuelve la última ingesta del punto de medición.
// Todos los errores devueltos son *AppError de processing/domain.
func (s *LatestIngestionService) GetLatestIngestion(ctx context.Context, measuringPointID int64,
) (*entities.LatestIngestion, error) {
	if measuringPointID <= 0 {
		s.processingLogger.Error(
			"[LatestIngestionService] Punto de medición inválido", "measuring_point_id", measuringPointID,
		)
		return nil, domain.ErrInvalidMeasuringPointID
	}

	s.processingLogger.Info(
		"[LatestIngestionService] Consultando la última ingesta",
		"measuring_point_id", measuringPointID,
	)

	measurements, err := s.measurementRepository.GetLatestMeasurements(ctx, measuringPointID)
	if err != nil {
		s.processingLogger.Error(
			"[LatestIngestionService] Error consultando la última ingesta",
			"measuring_point_id", measuringPointID,
			"error", err,
		)
		return nil, domain.ErrConsultingLatestMeasurements
	}

	if len(measurements) == 0 {
		s.processingLogger.Warn(
			"[LatestIngestionService] No hay mediciones para el punto de medición",
			"measuring_point_id", measuringPointID,
		)
		return nil, domain.ErrLatestMeasurementsNotFound
	}

	latest := &entities.LatestIngestion{
		MeasuringPointID: measuringPointID,
		Timestamp:        measurements[0].Timestamp,
		Measurements:     measurements,
	}

	s.processingLogger.Info(
		"[LatestIngestionService] Última ingesta obtenida",
		"measuring_point_id", latest.MeasuringPointID,
		"timestamp", latest.Timestamp.Format(time.RFC3339),
		"total_variables", len(latest.Measurements),
	)

	for _, measurement := range latest.Measurements {
		s.processingLogger.Info(
			"[LatestIngestionService] Variable",
			"metric_id", measurement.Metric.ID,
			"metric_key", measurement.Metric.Key,
			"value", measurement.Value,
			"unit", measurement.Metric.Unit,
		)
	}

	return latest, nil
}
