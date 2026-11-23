package bootstrap

import (
	"os"
	"strconv"
	"time"

	baselineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	baselineDomain "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/services"
	baselineMappers "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/mappers"
	baselineModels "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
	baselineRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/scheduling"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Métricas del catálogo que usa Baseline (sobrescribibles por entorno).
const (
	defaultMetricB1       = 12
	defaultMetricB2       = 13
	defaultMetricPlanitud = 18
	defaultTickMinute     = 10
)

// BaselineModule expone Baseline ya cableado.
type BaselineModule struct {
	// Recalculator lo usa cmd/baseline para recalcular (o dry-run) a demanda.
	Recalculator *baselineServices.RecalculationService
	// Backfiller lo usa cmd/baseline -backfill para cargar el histórico de
	// versiones de un punto que aún no tiene ninguna (o reemplazarlo con -rebuild).
	Backfiller *baselineServices.BackfillService
	// Exclusions mantiene baseline_exclusion_periods: lo usan la suscripción a
	// alert_management.alert_changed y cmd/baseline -seed-exclusions.
	Exclusions *baselineServices.ExclusionService
	// NewScheduler crea el scheduler de recálculo (tick horario + una ejecución al arrancar).
	NewScheduler func() *scheduling.RecalcScheduler

	bus eventbus.Bus
	log *logger.Logger
}

// SubscribeAlertChanged suscribe Baseline a alert_management.alert_changed
// para mantener baseline_exclusion_periods. Debe llamarse antes de arrancar
// los schedulers (si Alert Management está habilitado).
func (m *BaselineModule) SubscribeAlertChanged() (unsubscribe func()) {
	return subscribeBaselineExclusions(m.bus, m.Exclusions, m.log)
}

// NewBaselineModule migra el esquema de Baseline y cablea sus servicios.
// Solo lee raw_measurements y devices (de Ingestion). Los eventos conocidos
// salen de baseline_exclusion_periods (propia), que alimenta la suscripción a
// alert_changed (SubscribeAlertChanged) y, una vez, el seed desde alerts.
func NewBaselineModule(platform *Platform) (*BaselineModule, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logBaseline := logger.New("Baseline", "Info")

	// AutoMigrate crea tablas, FK, CHECKs, el UNIQUE (punto, versión), el índice
	// de vigencia y el índice parcial "una vigente por punto" (tags del modelo).
	if err := platform.DB.AutoMigrate(
		&baselineModels.BaselineVersionModel{},
		&baselineModels.BaselineComponentStatModel{},
		&baselineModels.ExclusionPeriodModel{},
	); err != nil {
		logDB.Error("falló la migración de Baseline", "error", err)
		return nil, err
	}

	cfg := baselineConfigFromEnv(logBaseline)
	metricIDs := baselineMetricIDsFromEnv(logBaseline)
	tickMinute := envMinute(logBaseline, "BASELINE_TICK_MINUTE", defaultTickMinute)

	historyReader := baselineRepositories.NewHistoryReader(platform.DB, metricIDs)
	baselineRepository := baselineRepositories.NewBaselineRepository(platform.DB)
	// Eventos conocidos = baseline_exclusion_periods (dato de entrada que
	// ningún rebuild borra), no alerts: así el rebuild no es circular.
	exclusionRepository := baselineRepositories.NewExclusionPeriodRepository(platform.DB)
	knownEvents := exclusionRepository
	exclusions := baselineServices.NewExclusionService(exclusionRepository,
		baselineRepositories.NewAlertIntervalReader(platform.DB), logBaseline)

	recalculator := baselineServices.NewRecalculationService(historyReader, baselineRepository, knownEvents, cfg, logBaseline, time.Now)
	replayService := baselineServices.NewReplayService(historyReader, knownEvents, cfg, logBaseline)
	backfiller := baselineServices.NewBackfillService(historyReader, baselineRepository, exclusionRepository, replayService,
		os.Getenv(EnvironmentEnv), logBaseline)

	return &BaselineModule{
		Recalculator: recalculator,
		Backfiller:   backfiller,
		Exclusions:   exclusions,
		bus:          platform.EventBus,
		log:          logBaseline,
		NewScheduler: func() *scheduling.RecalcScheduler {
			return scheduling.NewRecalcScheduler(recalculator, logBaseline, tickMinute)
		},
	}, nil
}

func baselineMetricIDsFromEnv(log *logger.Logger) baselineMappers.MetricIDs {
	return baselineMappers.MetricIDs{
		B1:       uint64(envInt(log, "BASELINE_METRIC_B1", defaultMetricB1)),
		B2:       uint64(envInt(log, "BASELINE_METRIC_B2", defaultMetricB2)),
		Planitud: uint64(envInt(log, "BASELINE_METRIC_PLANITUD", defaultMetricPlanitud)),
	}
}

// baselineConfigFromEnv parte de los valores cerrados con el analista y
// aplica los overrides de entorno. Un valor inválido se ignora con Warn.
func baselineConfigFromEnv(log *logger.Logger) baselineDomain.Config {
	cfg := baselineDomain.DefaultConfig()

	cfg.HistoryDays = envInt(log, "BASELINE_HISTORY_DAYS", cfg.HistoryDays)
	cfg.MinSignalRecords = envInt(log, "BASELINE_MIN_SIGNAL_RECORDS", cfg.MinSignalRecords)
	cfg.MinComponentWindows = envInt(log, "BASELINE_MIN_COMPONENT_WINDOWS", cfg.MinComponentWindows)
	cfg.MaxPasses = envInt(log, "BASELINE_MAX_PASSES", cfg.MaxPasses)
	cfg.ExclusionICutoff = envFloat(log, "BASELINE_EXCLUSION_I_CUTOFF", cfg.ExclusionICutoff)
	cfg.RecalcInterval = envDuration(log, "BASELINE_RECALC_INTERVAL", cfg.RecalcInterval)
	cfg.LowRecalcInterval = envDuration(log, "BASELINE_LOW_RECALC_INTERVAL", cfg.LowRecalcInterval)
	cfg.KnownEventMargin = envDuration(log, "BASELINE_KNOWN_EVENT_MARGIN", cfg.KnownEventMargin)

	return cfg
}

func envInt(log *logger.Logger, key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		log.Warn("variable de entorno inválida; se usa el valor por defecto", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}

func envFloat(log *logger.Logger, key string, def float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		log.Warn("variable de entorno inválida; se usa el valor por defecto", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}

func envDuration(log *logger.Logger, key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		log.Warn("variable de entorno inválida; se usa el valor por defecto", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}

// envMinute admite 0..59 (envInt rechaza el 0).
func envMinute(log *logger.Logger, key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 || v > 59 {
		log.Warn("variable de entorno inválida; se usa el valor por defecto", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}
