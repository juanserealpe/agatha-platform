package events

import (
	"context"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeEvaluator struct {
	calls []services.EvaluatePendingCommand
	err   error
}

func (f *fakeEvaluator) EvaluatePending(_ context.Context, cmd services.EvaluatePendingCommand,
) (services.EvaluationResult, error) {
	f.calls = append(f.calls, cmd)
	return services.EvaluationResult{}, f.err
}

func TestHandleBuildsCommand(t *testing.T) {
	seq := int64(81)
	watermark := time.Date(2026, 7, 28, 12, 45, 0, 0, time.UTC)
	from := watermark.Add(-15 * time.Minute)

	tests := []struct {
		name   string
		notice MeasurementsIngestedNotice
		want   services.EvaluatePendingCommand
	}{
		{
			name:   "con watermark usa floorHour(watermark) y trazabilidad",
			notice: MeasurementsIngestedNotice{MeasuringPointID: 11556, InsertedRecords: 21, Watermark: watermark, BatchSeq: &seq, From: &from},
			want: services.EvaluatePendingCommand{PointID: 11556, ReadyUntil: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
				Trigger: entities.TriggerEvent, BatchSeq: &seq, MaxSlots: 24, DataFrom: &from},
		},
		{
			name:   "sin watermark deja ReadyUntil en cero (se deriva de la BD)",
			notice: MeasurementsIngestedNotice{MeasuringPointID: 11556, InsertedRecords: 21},
			want:   services.EvaluatePendingCommand{PointID: 11556, Trigger: entities.TriggerEvent, MaxSlots: 24},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := &fakeEvaluator{}
			NewMeasurementsIngestedSubscriber(evaluator, 24, logger.New("ProcessingTest", "Error")).
				Handle(context.Background(), tt.notice)

			if len(evaluator.calls) != 1 {
				t.Fatalf("llamadas = %d", len(evaluator.calls))
			}
			got := evaluator.calls[0]
			if got.PointID != tt.want.PointID || !got.ReadyUntil.Equal(tt.want.ReadyUntil) || got.Trigger != tt.want.Trigger ||
				got.MaxSlots != tt.want.MaxSlots || got.BatchSeq != tt.want.BatchSeq || got.DataFrom != tt.want.DataFrom {
				t.Fatalf("comando = %+v, se esperaba %+v", got, tt.want)
			}
		})
	}
}

func TestHandleLogsErrorsWithoutPanicking(t *testing.T) {
	evaluator := &fakeEvaluator{err: domain.ErrInvalidMeasuringPointID}
	NewMeasurementsIngestedSubscriber(evaluator, 24, logger.New("ProcessingTest", "Error")).
		Handle(context.Background(), MeasurementsIngestedNotice{MeasuringPointID: 0})

	if len(evaluator.calls) != 1 {
		t.Fatalf("llamadas = %d", len(evaluator.calls))
	}
}
