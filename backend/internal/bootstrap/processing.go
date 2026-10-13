package bootstrap

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/ports"
	processingServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	processingMappers "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	processingModels "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
	processingRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/repositories"
	processingScheduling "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/scheduling"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// defaultReconcileMinute es el minuto (UTC) de la evaluación horaria.
const defaultReconcileMinute = 35

// ProcessingModule expone Processing ya cableado.
type ProcessingModule struct {
	// Evaluator lo usa cmd/processing para evaluar a demanda o cargar el histórico.
	Evaluator *processingServices.EvaluationService
	// NewScheduler crea la evaluación horaria (una ejecución al arrancar + minuto PROCESSING_RECONCILE_MINUTE).
	NewScheduler func() *processingScheduling.HourlyScheduler
}

// NewProcessingModule migra feature_evaluations y cablea Processing. Solo lee
// raw_measurements y devices (de Ingestion) y baseline_versions y
// baseline_component_stats (de Baseline), con sus propios modelos de lectura;
// no importa a ninguno de los dos módulos ni usa el bus.
func NewProcessingModule(platform *Platform) (*ProcessingModule, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logProcessing := logger.New("Processing", "Info")

	// AutoMigrate crea la tabla, el CHECK de window_hours y el índice único
	// (punto, ventana, hora). Los modelos de lectura de Baseline NO se migran.
	if err := platform.DB.AutoMigrate(&processingModels.FeatureEvaluationModel{}); err != nil {
		logDB.Error("falló la migración de Processing", "error", err)
		return nil, err
	}

	cfg, metricIDs := processingConfigFromEnv(logProcessing)
	reconcileMinute := envMinute(logProcessing, "PROCESSING_RECONCILE_MINUTE", defaultReconcileMinute)

	evaluator := processingServices.NewEvaluationService(
		processingRepositories.NewMeasurementReader(platform.DB, metricIDs),
		processingRepositories.NewEvaluationRepository(platform.DB),
		processingRepositories.NewBaselineReader(platform.DB),
		ports.ClockFunc(time.Now),
		cfg,
		logProcessing,
	)

	return &ProcessingModule{
		Evaluator: evaluator,
		NewScheduler: func() *processingScheduling.HourlyScheduler {
			return processingScheduling.NewHourlyScheduler(evaluator, logProcessing, reconcileMinute)
		},
	}, nil
}

// processingConfigFromEnv parte de los valores por defecto y aplica los
// overrides PROCESSING_*. Un valor inválido se ignora con Warn.
func processingConfigFromEnv(log *logger.Logger) (processingServices.Config, processingMappers.MetricIDs) {
	ids := processingMappers.MetricIDs{
		B1:       uint64(envInt(log, "PROCESSING_METRIC_B1", defaultMetricB1)),
		B2:       uint64(envInt(log, "PROCESSING_METRIC_B2", defaultMetricB2)),
		Planitud: uint64(envInt(log, "PROCESSING_METRIC_PLANITUD", defaultMetricPlanitud)),
	}

	cfg := processingServices.DefaultConfig()
	cfg.MaxSlotsReconciler = envInt(log, "PROCESSING_MAX_SLOTS_RECONCILER", cfg.MaxSlotsReconciler)
	cfg.InitialLookbackHours = envInt(log, "PROCESSING_INITIAL_LOOKBACK_HOURS", cfg.InitialLookbackHours)
	cfg.Grace = envDuration(log, "PROCESSING_GRACE", cfg.Grace)
	cfg.ConfigVersion = ids.ConfigVersion()

	return cfg, ids
}
