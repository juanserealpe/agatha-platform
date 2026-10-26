package bootstrap

import (
	"context"
	"net/http"
	"os"
	"time"

	"gorm.io/gorm"

	alertPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	alertServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	alertEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	alertEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/events"
	alertExport "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/export"
	alertHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/http"
	alertPersistence "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence"
	alertRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/repositories"
	alertScheduling "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/scheduling"
	ruleEngineContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// levelEvaluatedSubscription es el nombre de la suscripción de Alert Management.
const levelEvaluatedSubscription = "alert_management.level_evaluated"

// AlertManagementModule expone Alert Management ya cableado.
type AlertManagementModule struct {
	Processor *alertServices.ProcessingService
	// Replay es el catch-up (minuto 55) y el backfill (cmd/alert_management).
	Replay *alertServices.ReplayService

	handler *alertHTTP.AlertHandler
	bus     eventbus.Bus
	log     *logger.Logger
	minute  int
}

// MigrateAlertManagement migra el esquema y siembra la política global
// (idempotente). NewApplication construye Alert Management antes que Baseline
// para que la tabla alerts exista cuando Baseline arma sus eventos conocidos.
func MigrateAlertManagement(db *gorm.DB) error {
	logDB := logger.New("DataBaseInit", "Info")
	if err := alertPersistence.Migrate(db); err != nil {
		logDB.Error("falló la migración de Alert Management", "error", err)
		return err
	}
	if seeded, err := alertPersistence.SeedGlobalPolicy(db); err != nil {
		logDB.Error("falló el seed de la política de alertas", "error", err)
		return err
	} else if seeded {
		logDB.Info("versión global inicial de la política de alertas creada (seed)")
	}
	return nil
}

// NewAlertManagementModule migra y cablea Alert Management. Lee
// rule_evaluations (del Rule Engine) y devices (de Ingestion) con modelos
// propios; no importa a ningún módulo: la traducción del evento vive acá.
// No entrega avisos: publica alert_management.alert_notification en el bus
// (busAlertNotificationPublisher) para el módulo de notificaciones.
func NewAlertManagementModule(platform *Platform) (*AlertManagementModule, error) {
	log := logger.New("AlertManagement", "Info")
	if err := MigrateAlertManagement(platform.DB); err != nil {
		return nil, err
	}

	clock := alertPorts.ClockFunc(time.Now)
	store := alertRepositories.NewAlertStore(platform.DB)
	policies := alertRepositories.NewPolicyStore(platform.DB)

	processor := alertServices.NewProcessingService(store, policies, alertRepositories.NewDeviceDirectory(platform.DB),
		busAlertNotificationPublisher{bus: platform.EventBus}, clock, log)
	replay := alertServices.NewReplayService(processor, alertRepositories.NewLevelSource(platform.DB), store,
		alertServices.ReplayConfig{
			Environment: os.Getenv(EnvironmentEnv),
			BatchSize:   envInt(log, "ALERT_MANAGEMENT_BACKFILL_BATCH_SIZE", 500),
		}, log)

	handler := alertHTTP.NewAlertHandler(
		alertServices.NewQueryService(alertRepositories.NewAlertQueries(platform.DB), alertExport.NewXLSXWriter(), clock, log),
		alertServices.NewManagementService(store, clock, log),
		alertServices.NewPolicyService(policies, clock, log),
		log,
	)

	return &AlertManagementModule{
		Processor: processor,
		Replay:    replay,
		handler:   handler,
		bus:       platform.EventBus,
		log:       log,
		minute:    envMinute(log, "ALERT_MANAGEMENT_CATCHUP_MINUTE", alertScheduling.DefaultCatchUpMinute),
	}, nil
}

// RegisterRoutes registra /api/alerts. Sin autenticación todavía (RN-10).
func (m *AlertManagementModule) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// Runners es el catch-up horario (minuto 55).
func (m *AlertManagementModule) Runners() []Runner {
	return []Runner{
		{Name: "catch-up de alert management", Start: alertScheduling.NewCatchUpScheduler(m.Replay, m.log, m.minute).Start},
	}
}

// SubscribeLevelEvaluated suscribe Alert Management a rule_engine.level_evaluated.
// Debe llamarse antes de arrancar los schedulers.
func (m *AlertManagementModule) SubscribeLevelEvaluated() (unsubscribe func()) {
	return subscribeLevelEvaluated(m.bus, alertEvents.NewLevelEvaluatedSubscriber(m.Processor, m.log), m.log)
}

// levelEvaluatedHandler es lo que Alert Management hace con cada hora.
type levelEvaluatedHandler interface {
	Handle(ctx context.Context, notice alertEntities.LevelNotice)
}

// subscribeLevelEvaluated traduce el contrato del Rule Engine al Notice de
// Alert Management (que no importa al Rule Engine). El bus entrega en orden.
func subscribeLevelEvaluated(bus eventbus.Bus, handler levelEvaluatedHandler, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, ruleEngineContract.TopicLevelEvaluated,
		func(ctx context.Context, e ruleEngineContract.LevelEvaluated) {
			handler.Handle(ctx, toLevelNotice(e))
		},
		log,
		eventbus.WithName(levelEvaluatedSubscription),
	)
}

func toLevelNotice(e ruleEngineContract.LevelEvaluated) alertEntities.LevelNotice {
	n := alertEntities.LevelNotice{
		RuleEvaluationID:   e.RuleEvaluationID,
		MeasuringPointID:   e.MeasuringPointID,
		EventType:          alertEntities.EventType(e.EventType),
		EvaluatedAt:        e.EvaluatedAt.UTC(),
		Status:             e.Status,
		NoCalcReason:       e.NoCalcReason,
		IndicatorValue:     e.IndicatorValue,
		RawLevel:           alertEntities.Level(e.RawLevel),
		ComputedLevel:      alertEntities.Level(e.ComputedLevel),
		EpisodeLevel:       alertEntities.Level(e.EpisodeLevel),
		InEpisode:          e.InEpisode,
		EpisodeEvent:       alertEntities.EpisodeEvent(e.EpisodeEvent),
		Thresholds:         alertEntities.Cuts{Amarillo: e.Thresholds.Amarillo, Naranja: e.Thresholds.Naranja, Rojo: e.Thresholds.Rojo},
		Exit:               alertEntities.Cuts{Amarillo: e.Exit.Amarillo, Naranja: e.Exit.Naranja, Rojo: e.Exit.Rojo},
		ThresholdVersionID: e.ThresholdVersionID,
		HoursAbove:         e.HoursAbove,
		RunsAmarillo:       e.Runs.Amarillo,
		RunsNaranja:        e.Runs.Naranja,
		EscalationAmarillo: e.Escalation.AmarilloAfterHours,
		EscalationNaranja:  e.Escalation.NaranjaAfterHours,
		BelowRun:           e.BelowRun,
		LowConfidence:      e.LowConfidence,
	}
	for _, r := range e.Reasons {
		if r.Code == ruleEngineContract.ReasonLeakPersistence {
			n.PersistenceHours = r.Data.Hours
		}
	}
	return n
}

// busAlertNotificationPublisher es el adaptador del puerto
// NotificationPublisher hacia el bus. Vive acá porque Alert Management no puede
// importar shared/eventbus (depguard alert-management-isolation). Lo consumirá
// el módulo de notificaciones; sin suscriptores, publicar es un no-op del bus.
type busAlertNotificationPublisher struct {
	bus eventbus.Bus
}

func (p busAlertNotificationPublisher) PublishAlertNotification(ctx context.Context, n alertContract.AlertNotification) error {
	eventbus.PublishTyped(ctx, p.bus, alertContract.TopicAlertNotification, n)
	return nil
}
