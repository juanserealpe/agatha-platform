package events

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// PendingEvaluator es lo que el subscriber necesita del caso de uso UC-P1.
type PendingEvaluator interface {
	EvaluatePending(ctx context.Context, cmd services.EvaluatePendingCommand) (services.EvaluationResult, error)
}

// MeasurementsIngestedSubscriber recibe los avisos de lotes ingeridos y
// evalúa las horas que quedaron listas. Delgado: arma el comando y llama.
type MeasurementsIngestedSubscriber struct {
	evaluator PendingEvaluator
	maxSlots  int
	logger    *logger.Logger
}

// NewMeasurementsIngestedSubscriber crea el subscriber. maxSlots acota las
// horas por evento para respetar el timeout del bus.
func NewMeasurementsIngestedSubscriber(evaluator PendingEvaluator, maxSlots int, logger *logger.Logger,
) *MeasurementsIngestedSubscriber {
	return &MeasurementsIngestedSubscriber{evaluator: evaluator, maxSlots: maxSlots, logger: logger}
}

// Handle evalúa las horas pendientes hasta floorHour(watermark). Un evento no
// tiene a quién devolverle el error: se registra con su código y no se
// reintenta (el reconciliador horario es el reintento).
func (s *MeasurementsIngestedSubscriber) Handle(ctx context.Context, notice MeasurementsIngestedNotice) {
	s.logger.Info(
		"evento de mediciones ingeridas recibido",
		"measuring_point_id", notice.MeasuringPointID,
		"inserted_records", notice.InsertedRecords,
	)

	cmd := services.EvaluatePendingCommand{
		PointID:  notice.MeasuringPointID,
		Trigger:  entities.TriggerEvent,
		BatchSeq: notice.BatchSeq,
		MaxSlots: s.maxSlots,
		DataFrom: notice.From,
	}
	if !notice.Watermark.IsZero() {
		cmd.ReadyUntil = notice.Watermark.UTC().Truncate(time.Hour)
	}

	if _, err := s.evaluator.EvaluatePending(ctx, cmd); err != nil {
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
