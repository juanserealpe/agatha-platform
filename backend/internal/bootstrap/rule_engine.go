package bootstrap

import (
	"context"
	"net/http"
	"os"
	"time"

	processingContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	ruleEngineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	ruleEngineContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	ruleEngineEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	ruleEngineRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	ruleEngineEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/events"
	ruleEngineHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/http"
	ruleEnginePersistence "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence"
	ruleEngineMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	ruleEngineRepositoriesImpl "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/repositories"
	ruleEngineScheduling "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/scheduling"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// EnvironmentEnv es la variable con el entorno de despliegue. Con
// "production" el backfill del Rule Engine rechaza -rebuild.
const EnvironmentEnv = "AGATHA_ENV"

// indicatorEvaluatedSubscription es el nombre de la suscripción del Rule Engine.
const indicatorEvaluatedSubscription = "rule_engine.indicator_evaluated"

// RuleEngineModule expone el Rule Engine ya cableado.
type RuleEngineModule struct {
	// Thresholds son los casos de uso de umbrales (los expone RegisterRoutes).
	Thresholds *ruleEngineServices.ThresholdService
	// Simulator corre el histórico con umbrales propuestos sin guardar ni publicar.
	Simulator *ruleEngineServices.SimulationService
	// Evaluator interpreta cada hora (lo invoca la suscripción al bus).
	Evaluator *ruleEngineServices.EvaluationService
	// Backfiller lo usa cmd/rule_engine -backfill.
	Backfiller *ruleEngineServices.BackfillService
	// CatchUp interpreta, publicando, las horas posteriores a la última fila
	// que no llegaron por evento (red de seguridad de la suscripción).
	CatchUp *ruleEngineServices.CatchUpService
	// NewScheduler crea el catch-up horario (al arrancar + minuto RULE_ENGINE_CATCHUP_MINUTE).
	NewScheduler func() *ruleEngineScheduling.CatchUpScheduler

	bus eventbus.Bus
	log *logger.Logger
}

// ruleEngineDeps son las implementaciones de los puertos (en producción, GORM;
// en el test de punta a punta, memoria).
type ruleEngineDeps struct {
	thresholds  ruleEngineRepositories.ThresholdRepository
	evaluations ruleEngineRepositories.RuleEvaluationRepository
	source      ports.IndicatorSource
	clock       ports.Clock
	backfill    ruleEngineServices.BackfillConfig
	// catchUpMinute es el minuto (UTC) del catch-up horario.
	catchUpMinute int
}

// NewRuleEngineModule migra rule_threshold_versions y rule_evaluations (y
// siembra la versión 1 la primera vez) y cablea el Rule Engine. Lee
// feature_evaluations con su propio modelo y publica sus eventos en el bus de
// la Platform a través de busLevelEventPublisher. No suscribe: lo hace
// SubscribeIndicatorEvaluated (cmd/rule_engine no lo necesita), y el catch-up
// horario lo arranca NewApplication como Runner.
func NewRuleEngineModule(platform *Platform) (*RuleEngineModule, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logRuleEngine := logger.New("RuleEngine", "Info")

	if err := ruleEnginePersistence.Migrate(platform.DB); err != nil {
		logDB.Error("falló la migración del Rule Engine", "error", err)
		return nil, err
	}

	seeded, err := ruleEnginePersistence.SeedInitialThresholds(platform.DB)
	if err != nil {
		logDB.Error("falló el seed de umbrales del Rule Engine", "error", err)
		return nil, err
	}
	if seeded > 0 {
		logRuleEngine.Info("versión 1 de umbrales creada para los puntos existentes (seed)", "points", seeded)
	}

	return wireRuleEngine(platform.EventBus, ruleEngineDeps{
		thresholds:  ruleEngineRepositoriesImpl.NewThresholdRepository(platform.DB),
		evaluations: ruleEngineRepositoriesImpl.NewRuleEvaluationRepository(platform.DB),
		source:      ruleEngineRepositoriesImpl.NewIndicatorReader(platform.DB),
		clock:       ports.ClockFunc(time.Now),
		backfill: ruleEngineServices.BackfillConfig{
			Environment: os.Getenv(EnvironmentEnv),
			BatchSize:   envInt(logRuleEngine, "RULE_ENGINE_BACKFILL_BATCH_SIZE", 500),
		},
		catchUpMinute: envMinute(logRuleEngine, "RULE_ENGINE_CATCHUP_MINUTE", ruleEngineScheduling.DefaultCatchUpMinute),
	}, logRuleEngine), nil
}

func wireRuleEngine(bus eventbus.Bus, deps ruleEngineDeps, log *logger.Logger) *RuleEngineModule {
	thresholds := ruleEngineServices.NewThresholdService(deps.thresholds, deps.clock, log)
	evaluator := ruleEngineServices.NewEvaluationService(
		deps.thresholds,
		deps.evaluations,
		thresholds,
		deps.source,
		busLevelEventPublisher{bus: bus},
		deps.clock,
		log,
	)

	catchUp := ruleEngineServices.NewCatchUpService(evaluator, deps.source, deps.evaluations, deps.backfill.BatchSize, log)

	return &RuleEngineModule{
		Thresholds: thresholds,
		Simulator:  ruleEngineServices.NewSimulationService(deps.thresholds, deps.source, memorySandbox, deps.clock, log),
		Evaluator:  evaluator,
		Backfiller: ruleEngineServices.NewBackfillService(evaluator, deps.source, deps.evaluations, deps.backfill, log),
		CatchUp:    catchUp,
		NewScheduler: func() *ruleEngineScheduling.CatchUpScheduler {
			return ruleEngineScheduling.NewCatchUpScheduler(catchUp, log, deps.catchUpMinute)
		},
		bus: bus,
		log: log,
	}
}

// memorySandbox es el sandbox de la simulación: repositorios en memoria nuevos
// en cada corrida (nunca toca la BD).
func memorySandbox(seed ruleEngineEntities.ThresholdVersion) (ruleEngineRepositories.ThresholdRepository, ruleEngineRepositories.RuleEvaluationRepository) {
	return ruleEngineMemory.NewThresholdRepository(seed), ruleEngineMemory.NewRuleEvaluationRepository()
}

// RegisterRoutes registra los endpoints de umbrales (/api/rule-engine). Sin
// autenticación de usuarios todavía: created_by viaja en el cuerpo.
func (m *RuleEngineModule) RegisterRoutes(mux *http.ServeMux) {
	ruleEngineHTTP.NewThresholdHandler(m.Thresholds, m.Simulator, m.log).RegisterRoutes(mux)
}

// SubscribeIndicatorEvaluated suscribe el Rule Engine a processing.indicator_evaluated.
// Debe llamarse antes de arrancar el scheduler de Processing: un evento sin
// suscriptores se descarta.
func (m *RuleEngineModule) SubscribeIndicatorEvaluated() (unsubscribe func()) {
	return subscribeIndicatorEvaluated(m.bus, ruleEngineEvents.NewIndicatorEvaluatedSubscriber(m.Evaluator, m.log), m.log)
}

// indicatorEvaluatedHandler es lo que el Rule Engine hace con cada ventana
// evaluada (en producción, *IndicatorEvaluatedSubscriber).
type indicatorEvaluatedHandler interface {
	Handle(ctx context.Context, notice ruleEngineServices.IndicatorEvaluatedNotice)
}

// subscribeIndicatorEvaluated traduce el contrato de Processing al Notice del
// Rule Engine (que no importa a Processing) y lo entrega al handler. El bus
// entrega en orden (un worker FIFO por suscripción): las horas de un punto se
// interpretan en el orden en que se publicaron.
func subscribeIndicatorEvaluated(bus eventbus.Bus, handler indicatorEvaluatedHandler, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, processingContract.TopicIndicatorEvaluated,
		func(ctx context.Context, e processingContract.IndicatorEvaluated) {
			handler.Handle(ctx, toIndicatorEvaluatedNotice(e))
		},
		log,
		eventbus.WithName(indicatorEvaluatedSubscription),
	)
}

func toIndicatorEvaluatedNotice(e processingContract.IndicatorEvaluated) ruleEngineServices.IndicatorEvaluatedNotice {
	return ruleEngineServices.IndicatorEvaluatedNotice{
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

// busLevelEventPublisher es el adaptador del puerto LevelEventPublisher hacia
// el bus. Vive acá porque el Rule Engine no puede importar shared/eventbus
// (depguard rule-engine-isolation). Lo consume Alert Management
// (subscribeLevelEvaluated); sin ese módulo habilitado, publicar es un no-op del bus.
type busLevelEventPublisher struct {
	bus eventbus.Bus
}

func (p busLevelEventPublisher) PublishLevelEvaluated(ctx context.Context, e ruleEngineContract.LevelEvaluated) error {
	eventbus.PublishTyped(ctx, p.bus, ruleEngineContract.TopicLevelEvaluated, e)
	return nil
}
