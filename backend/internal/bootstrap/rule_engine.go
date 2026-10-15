package bootstrap

import (
	"context"

	processingContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	ruleEngineEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// indicatorEvaluatedHandler es lo que el Rule Engine hace con cada ventana
// evaluada (en producción, *IndicatorEvaluatedSubscriber).
type indicatorEvaluatedHandler interface {
	Handle(ctx context.Context, notice ruleEngineEvents.IndicatorEvaluatedNotice)
}

// NewRuleEngineModule suscribe el Rule Engine a processing.indicator_evaluated.
// Debe llamarse antes de arrancar el scheduler de Processing: un evento sin
// suscriptores se descarta. Hoy el Rule Engine solo registra el evento en el log.
func NewRuleEngineModule(platform *Platform) {
	logRuleEngine := logger.New("RuleEngine", "Info")

	subscribeIndicatorEvaluated(
		platform.EventBus,
		ruleEngineEvents.NewIndicatorEvaluatedSubscriber(logRuleEngine),
		logRuleEngine,
	)
}

// subscribeIndicatorEvaluated traduce el contrato de Processing al Notice del
// Rule Engine (que no importa a Processing) y lo entrega al handler.
func subscribeIndicatorEvaluated(bus eventbus.Bus, handler indicatorEvaluatedHandler, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, processingContract.TopicIndicatorEvaluated,
		func(ctx context.Context, e processingContract.IndicatorEvaluated) {
			handler.Handle(ctx, toIndicatorEvaluatedNotice(e))
		},
		log,
		eventbus.WithName("rule_engine.indicator_evaluated"),
	)
}

func toIndicatorEvaluatedNotice(e processingContract.IndicatorEvaluated) ruleEngineEvents.IndicatorEvaluatedNotice {
	return ruleEngineEvents.IndicatorEvaluatedNotice{
		MeasuringPointID: e.MeasuringPointID,
		WindowHours:      e.WindowHours,
		EvaluatedAt:      e.EvaluatedAt,
		Status:           e.Status,
		NoCalcReason:     e.NoCalcReason,
		I:                e.I,
		LowConfidence:    e.LowConfidence,
		Trigger:          e.Trigger,
		OccurredAt:       e.OccurredAt,
	}
}
