package bootstrap

import (
	"context"

	ingestionContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	processingEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// processingSubscriptionName identifica la suscripción de Processing en logs y Stats.
const processingSubscriptionName = "processing.measurements_ingested"

// ProcessingModule representa a Processing inicializado sobre el bus de la Platform.
type ProcessingModule struct {
	// Unsubscribe cancela las suscripciones de Processing. En el apagado normal
	// no hace falta: Platform.Close drena y cierra las suscripciones.
	Unsubscribe func()
}

// NewProcessingModule registra los subscribers de Processing en el bus.
// Debe llamarse ANTES de que Ingestion empiece a publicar: un evento sin
// suscriptores se descarta en silencio.
func NewProcessingModule(platform *Platform) *ProcessingModule {
	return &ProcessingModule{
		Unsubscribe: registerProcessingSubscribers(platform.EventBus, logger.New("Processing", "Info")),
	}
}

// registerProcessingSubscribers conecta Processing a los eventos de Ingestion.
//
// Es el único punto que conoce ambos módulos: traduce el contrato de Ingestion
// al tipo propio de Processing (capa anticorrupción).
func registerProcessingSubscribers(bus eventbus.Bus, log *logger.Logger) (unsubscribe func()) {
	subscriber := processingEvents.NewMeasurementsIngestedSubscriber(log)

	return eventbus.SubscribeTyped(
		bus,
		ingestionContract.TopicMeasurementsIngested,
		func(ctx context.Context, event ingestionContract.MeasurementsIngested) {
			subscriber.Handle(ctx, toMeasurementsIngestedNotice(event))
		},
		log,
		eventbus.WithName(processingSubscriptionName),
	)
}

// toMeasurementsIngestedNotice traduce el evento de Ingestion al tipo de Processing.
func toMeasurementsIngestedNotice(event ingestionContract.MeasurementsIngested) processingEvents.MeasurementsIngestedNotice {
	return processingEvents.MeasurementsIngestedNotice{
		MeasuringPointID: event.MeasuringPointID,
		Source:           event.Source,
		From:             event.From,
		To:               event.To,
		Watermark:        event.Watermark,
		BatchSeq:         event.BatchSeq,
		InsertedRecords:  event.InsertedRecords,
	}
}
