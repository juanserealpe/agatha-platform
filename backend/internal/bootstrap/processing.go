package bootstrap

import (
	"context"

	ingestionContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	processingServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	processingEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/events"
	processingRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/repositories"
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
	logProcessing := logger.New("Processing", "Info")

	measurementRepository := processingRepositories.NewMeasurementRepository(platform.DB)
	latestIngestionService := processingServices.NewLatestIngestionService(measurementRepository, logProcessing)
	subscriber := processingEvents.NewMeasurementsIngestedSubscriber(latestIngestionService, logProcessing)

	return &ProcessingModule{
		Unsubscribe: registerProcessingSubscribers(platform.EventBus, subscriber, logProcessing),
	}
}

// registerProcessingSubscribers conecta Processing a los eventos de Ingestion.
//
// Es el único punto que conoce ambos módulos: traduce el contrato de Ingestion
// al tipo propio de Processing (capa anticorrupción).
func registerProcessingSubscribers(
	bus eventbus.Bus,
	subscriber *processingEvents.MeasurementsIngestedSubscriber,
	log *logger.Logger,
) (unsubscribe func()) {

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
		InsertedRecords:  event.InsertedRecords,
	}
}
