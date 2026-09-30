package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// MeasurementsIngestedSubscriber recibe los avisos de lotes ingeridos.
// Por ahora solo los registra en log (observabilidad hasta que exista el cálculo).
type MeasurementsIngestedSubscriber struct {
	logger *logger.Logger
}

// NewMeasurementsIngestedSubscriber crea el subscriber con su logger.
func NewMeasurementsIngestedSubscriber(logger *logger.Logger) *MeasurementsIngestedSubscriber {
	return &MeasurementsIngestedSubscriber{logger: logger}
}

// Handle registra la recepción del aviso.
func (s *MeasurementsIngestedSubscriber) Handle(_ context.Context, notice MeasurementsIngestedNotice) {
	s.logger.Info(
		"evento de mediciones ingeridas recibido",
		"measuring_point_id", notice.MeasuringPointID,
		"source", notice.Source,
		"batch_seq", notice.BatchSeq,
		"inserted_records", notice.InsertedRecords,
		"from", notice.From,
		"to", notice.To,
		"watermark", notice.Watermark,
	)
}
