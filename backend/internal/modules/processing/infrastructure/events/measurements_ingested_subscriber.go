package events

import (
	"context"
	"errors"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// MeasurementsIngestedSubscriber recibe los avisos de lotes ingeridos y lee
// de la BD la última ingesta del punto de medición.
type MeasurementsIngestedSubscriber struct {
	latestIngestionService *services.LatestIngestionService
	logger                 *logger.Logger
}

// NewMeasurementsIngestedSubscriber crea el subscriber con su servicio y logger.
func NewMeasurementsIngestedSubscriber(
	latestIngestionService *services.LatestIngestionService,
	logger *logger.Logger,
) *MeasurementsIngestedSubscriber {
	return &MeasurementsIngestedSubscriber{
		latestIngestionService: latestIngestionService,
		logger:                 logger,
	}
}

// Handle registra la recepción del aviso y consulta la última ingesta.
// Un evento no tiene a quién devolverle el error: se registra en log con su código.
func (s *MeasurementsIngestedSubscriber) Handle(ctx context.Context, notice MeasurementsIngestedNotice) {
	s.logger.Info(
		"evento de mediciones ingeridas recibido",
		"measuring_point_id", notice.MeasuringPointID,
		"inserted_records", notice.InsertedRecords,
	)

	_, err := s.latestIngestionService.GetLatestIngestion(ctx, notice.MeasuringPointID)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			s.logger.Error(
				"no fue posible procesar el evento de mediciones ingeridas",
				"measuring_point_id", notice.MeasuringPointID,
				"code", appErr.Code,
				"error", appErr.Message,
			)
			return
		}

		s.logger.Error(
			"error inesperado procesando el evento de mediciones ingeridas",
			"measuring_point_id", notice.MeasuringPointID,
			"error", err,
		)
	}
}
