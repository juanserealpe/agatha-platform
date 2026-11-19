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

// levelEvaluatedSubscription y motionEvaluatedSubscription son las suscripciones de Alert Management.
const (
	levelEvaluatedSubscription  = "alert_management.level_evaluated"
	motionEvaluatedSubscription = "alert_management.motion_evaluated"
)

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

	motions := alertRepositories.NewMotionSource(platform.DB)
	processor := alertServices.NewProcessingService(store, policies, alertRepositories.NewDeviceDirectory(platform.DB),
		busAlertNotificationPublisher{bus: platform.EventBus}, clock, log).
		WithChangePublisher(busAlertChangePublisher{bus: platform.EventBus}).
		WithMotionSource(motions)
	replay := alertServices.NewReplayService(processor, alertRepositories.NewLevelSource(platform.DB), store,
		alertServices.ReplayConfig{
			Environment: os.Getenv(EnvironmentEnv),
			BatchSize:   envInt(log, "ALERT_MANAGEMENT_BACKFILL_BATCH_SIZE", 500),
		}, log).WithMotionSource(motions)

	handler := alertHTTP.NewAlertHandler(
		alertServices.NewQueryService(alertRepositories.NewAlertQueries(platform.DB), alertExport.NewXLSXWriter(), clock, log),
		alertServices.NewManagementService(store, clock, log).WithChangePublisher(busAlertChangePublisher{bus: platform.EventBus}),
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

// SubscribeMotionEvaluated suscribe Alert Management a rule_engine.motion_evaluated.
// Debe llamarse antes de arrancar los schedulers.
func (m *AlertManagementModule) SubscribeMotionEvaluated() (unsubscribe func()) {
	return subscribeMotionEvaluated(m.bus, alertEvents.NewMotionEvaluatedSubscriber(m.Processor, m.log), m.log)
}

// motionEvaluatedHandler es lo que Alert Management hace con cada hora de movimiento.
type motionEvaluatedHandler interface {
	Handle(ctx context.Context, notice alertEntities.MotionNotice)
}

// subscribeMotionEvaluated traduce el contrato de movimiento del Rule Engine
// al MotionNotice de Alert Management. El bus entrega en orden.
func subscribeMotionEvaluated(bus eventbus.Bus, handler motionEvaluatedHandler, log *logger.Logger) (unsubscribe func()) {
	return eventbus.SubscribeTyped(bus, ruleEngineContract.TopicMotionEvaluated,
		func(ctx context.Context, e ruleEngineContract.MotionEvaluated) {
			handler.Handle(ctx, toMotionNotice(e))
		},
		log,
		eventbus.WithName(motionEvaluatedSubscription),
	)
}

// toMotionNotice traduce rule_engine.motion_evaluated v1. El evento solo se
// publica con alerts_enabled (el modo sombra no llega al bus).
func toMotionNotice(e ruleEngineContract.MotionEvaluated) alertEntities.MotionNotice {
	n := alertEntities.MotionNotice{
		MotionEvaluationID: e.MotionEvaluationID,
		MeasuringPointID:   e.MeasuringPointID,
		EvaluatedAt:        e.EvaluatedAt.UTC(),
		Status:             e.Status,
		MotionState:        e.MotionState,
		TempState:          e.TempState,
		TempMode:           e.TempMode,
		ComputedLevel:      alertEntities.Level(e.ComputedLevel),
		InEpisode:          e.InEpisode,
		EpisodeEvent:       alertEntities.EpisodeEvent(e.EpisodeEvent),
		ShiftPitch:         copyFloat(e.Shift.Pitch),
		ShiftRoll:          copyFloat(e.Shift.Roll),
		ShiftStartedAt:     copyTime(e.Shift.StartedAt),
		GapFrom:            copyTime(e.Shift.GapFrom),
		GapTo:              copyTime(e.Shift.GapTo),
		TempStableFrom:     copyTime(e.TempStable.From),
		TempStableTo:       copyTime(e.TempStable.To),
		ThresholdVersionID: e.ThresholdVersionID,
		AlertsEnabled:      true,
	}
	for _, r := range e.Reasons {
		n.Reasons = append(n.Reasons, alertEntities.ParseMotionReason(r.Code, r.Data))
	}
	return n
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
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
		DPl:                copyFloat(e.DPl),
		DPlMin:             copyFloat(e.DPlMin),
	}
	for _, r := range e.Reasons {
		switch r.Code {
		case ruleEngineContract.ReasonLeakPersistence:
			n.PersistenceHours = r.Data.Hours
		case ruleEngineContract.ReasonNaranjaNotConfirmed:
			n.LimitedFrom = alertEntities.Level(r.Data.From)
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

// busAlertChangePublisher es el adaptador del puerto AlertChangePublisher
// hacia el bus, por la misma razón que busAlertNotificationPublisher.
type busAlertChangePublisher struct {
	bus eventbus.Bus
}

func (p busAlertChangePublisher) PublishAlertChanged(ctx context.Context, c alertContract.AlertChanged) error {
	eventbus.PublishTyped(ctx, p.bus, alertContract.TopicAlertChanged, c)
	return nil
}

// copyFloat copia el puntero: el aviso no comparte memoria con el evento.
func copyFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
