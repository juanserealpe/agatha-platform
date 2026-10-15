package bootstrap

import (
	"context"
	"testing"
	"time"

	processingContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	ruleEngineEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type noticeRecorder chan ruleEngineEvents.IndicatorEvaluatedNotice

func (r noticeRecorder) Handle(_ context.Context, n ruleEngineEvents.IndicatorEvaluatedNotice) {
	r <- n
}

// De punta a punta con el bus real: Processing publica su contrato y el Rule
// Engine recibe su propio Notice traducido por bootstrap.
func TestSubscribeRuleEngine_TranslatesIndicatorEvaluated(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	received := make(noticeRecorder, 1)
	subscribeIndicatorEvaluated(bus, received, logger.New("RuleEngineTest", "Error"))

	i := 0.31
	event := processingContract.IndicatorEvaluated{
		MeasuringPointID: 11556,
		WindowHours:      6,
		EvaluatedAt:      time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Status:           "CALCULATED",
		NoCalcReason:     "",
		I:                &i,
		LowConfidence:    true,
		Trigger:          "RECONCILER",
		OccurredAt:       time.Date(2026, 10, 15, 12, 35, 0, 0, time.UTC),
		SchemaVersion:    processingContract.SchemaVersion,
	}
	eventbus.PublishTyped(context.Background(), bus, processingContract.TopicIndicatorEvaluated, event)

	select {
	case got := <-received:
		want := ruleEngineEvents.IndicatorEvaluatedNotice{
			MeasuringPointID: event.MeasuringPointID,
			WindowHours:      event.WindowHours,
			EvaluatedAt:      event.EvaluatedAt,
			Status:           event.Status,
			NoCalcReason:     event.NoCalcReason,
			I:                event.I,
			LowConfidence:    event.LowConfidence,
			Trigger:          event.Trigger,
			OccurredAt:       event.OccurredAt,
		}
		if got != want {
			t.Fatalf("notice = %+v, se esperaba %+v", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el Rule Engine no recibió el evento")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bus.Close(ctx); err != nil {
		t.Fatal(err)
	}

	for _, s := range bus.Stats() {
		if s.Topic == processingContract.TopicIndicatorEvaluated && s.Name != "rule_engine.indicator_evaluated" {
			t.Errorf("nombre de suscripción = %q", s.Name)
		}
	}
}
