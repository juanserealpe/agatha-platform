package bootstrap

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline"
	ingestionContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/ports"
	processingServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	processingEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/events"
	processingMappers "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	processingModels "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
	processingRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/repositories"
	processingScheduling "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/scheduling"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// processingSubscriptionName identifica la suscripción de Processing en logs y Stats.
const processingSubscriptionName = "processing.measurements_ingested"

// defaultReconcileMinute es el minuto (UTC) del reconciliador horario.
const defaultReconcileMinute = 35

// ProcessingModule representa a Processing inicializado sobre la Platform.
type ProcessingModule struct {
	// Unsubscribe cancela las suscripciones de Processing. En el apagado normal
	// no hace falta: Platform.Close drena y cierra las suscripciones.
	Unsubscribe func()
	// NewScheduler crea el reconciliador horario (una ejecución al arrancar + minuto PROCESSING_RECONCILE_MINUTE).
	NewScheduler func() *processingScheduling.HourlyScheduler
}

// NewProcessingModule migra feature_evaluations, cablea Processing sobre la
// fachada de Baseline y registra su subscriber en el bus. Debe llamarse ANTES
// de que Ingestion empiece a publicar: un evento sin suscriptores se descarta.
func NewProcessingModule(platform *Platform, baselineModule *BaselineModule) (*ProcessingModule, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logProcessing := logger.New("Processing", "Info")

	// AutoMigrate crea la tabla, el CHECK de window_hours y el índice único
	// (punto, ventana, hora) declarados en los tags del modelo.
	if err := platform.DB.AutoMigrate(&processingModels.FeatureEvaluationModel{}); err != nil {
		logDB.Error("falló la migración de Processing", "error", err)
		return nil, err
	}

	cfg, metricIDs := processingConfigFromEnv(logProcessing)
	reconcileMinute := envMinute(logProcessing, "PROCESSING_RECONCILE_MINUTE", defaultReconcileMinute)

	service := processingServices.NewEvaluationService(
		processingRepositories.NewMeasurementReader(platform.DB, metricIDs),
		processingRepositories.NewEvaluationRepository(platform.DB),
		&baselineProviderAdapter{facade: baselineModule.Facade},
		ports.ClockFunc(time.Now),
		cfg,
		logProcessing,
	)

	subscriber := processingEvents.NewMeasurementsIngestedSubscriber(service, cfg.MaxSlotsEvent, logProcessing)

	return &ProcessingModule{
		Unsubscribe: registerProcessingSubscribers(platform.EventBus, subscriber, logProcessing),
		NewScheduler: func() *processingScheduling.HourlyScheduler {
			return processingScheduling.NewHourlyScheduler(service, logProcessing, reconcileMinute)
		},
	}, nil
}

// processingConfigFromEnv parte de los valores del spec y aplica los
// overrides PROCESSING_*. Un valor inválido se ignora con Warn.
func processingConfigFromEnv(log *logger.Logger) (processingServices.Config, processingMappers.MetricIDs) {
	ids := processingMappers.MetricIDs{
		B1:       uint64(envInt(log, "PROCESSING_METRIC_B1", defaultMetricB1)),
		B2:       uint64(envInt(log, "PROCESSING_METRIC_B2", defaultMetricB2)),
		Planitud: uint64(envInt(log, "PROCESSING_METRIC_PLANITUD", defaultMetricPlanitud)),
	}

	cfg := processingServices.DefaultConfig()
	cfg.MaxSlotsEvent = envInt(log, "PROCESSING_MAX_SLOTS_EVENT", cfg.MaxSlotsEvent)
	cfg.MaxSlotsReconciler = envInt(log, "PROCESSING_MAX_SLOTS_RECONCILER", cfg.MaxSlotsReconciler)
	cfg.InitialLookbackHours = envInt(log, "PROCESSING_INITIAL_LOOKBACK_HOURS", cfg.InitialLookbackHours)
	cfg.Grace = envDuration(log, "PROCESSING_GRACE", cfg.Grace)
	cfg.ConfigVersion = ids.ConfigVersion()

	return cfg, ids
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
// Watermark se toma de event.Watermark: Ingestion persiste last_ingested_at como
// MAX(timestamp) de las muestras del lote (no el fin del rango pedido a DataOS),
// así que watermark ≥ t garantiza que existe un registro con timestamp ≥ t.
// Campos en cero (evento sin datos de lote) quedan vacíos y Processing deriva la
// hora lista de la BD.
func toMeasurementsIngestedNotice(event ingestionContract.MeasurementsIngested) processingEvents.MeasurementsIngestedNotice {
	notice := processingEvents.MeasurementsIngestedNotice{
		MeasuringPointID: event.MeasuringPointID,
		InsertedRecords:  event.InsertedRecords,
		Watermark:        event.Watermark.UTC(),
	}
	if event.BatchSeq > 0 {
		batchSeq := event.BatchSeq
		notice.BatchSeq = &batchSeq
	}
	if !event.From.IsZero() {
		from := event.From.UTC()
		notice.From = &from
	}
	return notice
}

// baselineProviderAdapter implementa el port BaselineProvider de Processing
// sobre la fachada pública de Baseline. Sin caché: es una consulta indexada
// y la línea base cambia como mucho una vez por día.
type baselineProviderAdapter struct {
	facade baseline.Facade
}

func (a *baselineProviderAdapter) GetEffective(ctx context.Context, pointID int64, at time.Time,
) (entities.BaselineSnapshot, error) {
	effective, err := a.facade.GetEffectiveBaseline(ctx, pointID, at)
	if err != nil {
		return entities.BaselineSnapshot{}, err
	}
	return toBaselineSnapshot(effective), nil
}

// toBaselineSnapshot traduce 1:1 el contrato de Baseline a la vista de Processing.
func toBaselineSnapshot(eb baseline.EffectiveBaseline) entities.BaselineSnapshot {
	snap := entities.BaselineSnapshot{
		Available:        eb.Status == baseline.StatusAvailable,
		VersionID:        eb.VersionID,
		Version:          eb.Version,
		ValidFrom:        eb.ValidFrom.UTC(),
		Confidence:       eb.Confidence,
		Stats:            map[int]features.ComponentStats{},
		Flags:            append([]string{}, eb.Flags...),
		AlgorithmVersion: eb.AlgorithmVersion,
	}

	if eb.DeviceFirstSeenAt != nil {
		first := eb.DeviceFirstSeenAt.UTC()
		snap.FirstSeenAt = &first
	}

	if s := eb.Signal; s != nil {
		snap.Signal = features.Signal{MedianB2: s.MedianB2, ThetaD: s.ThetaD, DeltaMin: s.DeltaMin}
		if s.P90Planitud != nil {
			snap.Signal.P90Planitud, snap.Signal.HasP90Planitud = *s.P90Planitud, true
		}
		if s.MinElevatedB2 != nil {
			snap.Signal.MinElevatedB2, snap.Signal.HasMinElevated = *s.MinElevatedB2, true
		}
	}

	for _, c := range eb.Components {
		st := snap.Stats[c.WindowHours]
		stat := features.Stat{Median: c.Median, MAD: c.MAD, N: c.NWindows}

		switch c.Component {
		case features.ComponentD:
			st.D = stat
		case features.ComponentP:
			st.P = stat
		case features.ComponentEx:
			st.Ex = stat
		case features.ComponentC:
			st.C, st.HasC = stat, true
		}

		snap.Stats[c.WindowHours] = st
	}

	return snap
}
