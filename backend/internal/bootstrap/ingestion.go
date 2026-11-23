package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/mapping"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/scheduler"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const dataOSBaseURL = "https://app.dataos.dataflowlabs.io"

// ingestionTimezone es la zona con la que se interpretan los timestamps sin
// offset de DataOS y del callback, y con la que se alinea el scheduler.
const ingestionTimezone = "America/Bogota"

// Configuración del catch-up de DataOS (ver catchUpSettings).
const (
	// CatchUpLookbackEnv es la ventana en la que se buscan huecos internos
	// (time.ParseDuration; por defecto 168h). Si el último dato es más viejo,
	// la búsqueda arranca en él.
	CatchUpLookbackEnv = "INGESTION_CATCHUP_LOOKBACK"

	defaultCatchUpLookback = 7 * 24 * time.Hour
	// Registros cada 15 min: más de 20 min sin datos es un hueco.
	catchUpMaxStep    = 20 * time.Minute
	catchUpJoinWithin = 6 * time.Hour
	// DataOS (Supabase) falla con rangos mayores a 5 días.
	catchUpPartition = 5 * 24 * time.Hour
	schedulerDelay   = 50 * time.Second
)

// IngestionModule expone los servicios de Ingestion ya cableados.
type IngestionModule struct {
	IngestService  *services.IngestService
	CatchUpService func(cfg services.CatchUpConfig) *services.CatchUpService
	Logger         *logger.Logger
}

// catchUpPlan es la configuración efectiva del catch-up según la vía de ingesta.
type catchUpPlan struct {
	interval time.Duration
	cfg      services.CatchUpConfig
}

// catchUpSettings decide cómo corre el catch-up:
//   - sin callback (PC): DataOS es la única vía; cada 15 min y hasta ahora.
//   - con callback (VPS): respaldo; cada hora y sin tocar la última hora, que
//     la entrega el callback (envío ~4 min después y reintentos).
//
// En los dos casos corre una vez al arrancar, antes que los demás schedulers.
func catchUpSettings(log *logger.Logger, withCallback bool) catchUpPlan {
	plan := catchUpPlan{
		interval: 15 * time.Minute,
		cfg: services.CatchUpConfig{
			Lookback:   envDuration(log, CatchUpLookbackEnv, defaultCatchUpLookback),
			MaxStep:    catchUpMaxStep,
			JoinWithin: catchUpJoinWithin,
			Partition:  catchUpPartition,
		},
	}
	if withCallback {
		plan.interval = time.Hour
		plan.cfg.MinAge = time.Hour
	}
	return plan
}

// NewScheduler crea el scheduler de catch-up. withCallback indica que el
// callback también está habilitado (DataOS pasa a ser respaldo).
func (m *IngestionModule) NewScheduler(withCallback bool) (*scheduler.IngestionScheduler, error) {
	location, err := loadIngestionLocation()
	if err != nil {
		return nil, err
	}

	plan := catchUpSettings(m.Logger, withCallback)
	mode := "principal"
	if withCallback {
		mode = "respaldo del callback"
	}
	m.Logger.Info("catch-up de DataOS configurado", "modo", mode, "cada", plan.interval,
		"ventana", plan.cfg.Lookback, "sin_tocar_ultimos", plan.cfg.MinAge)

	return scheduler.NewIngestionScheduler(m.CatchUpService(plan.cfg), m.Logger, plan.interval, location, schedulerDelay), nil
}

// IngestionApplication es el proceso de Ingestion aislado (cmd/ingestion,
// cmd/scheduler): Platform + Baseline + Ingestion.
type IngestionApplication struct {
	*IngestionModule
	Platform *Platform
	// Baseline queda compuesto; cmd/scheduler arranca su scheduler.
	Baseline *BaselineModule
}

// Close cierra el bus de eventos drenando los eventos pendientes hasta el deadline de ctx.
func (a *IngestionApplication) Close(ctx context.Context) error {
	return a.Platform.Close(ctx)
}

// NewIngestionApplication compone la Platform con Baseline e Ingestion.
func NewIngestionApplication() (*IngestionApplication, error) {
	platform, err := NewPlatform()
	if err != nil {
		return nil, err
	}

	baselineModule, err := NewBaselineModule(platform)
	if err != nil {
		return nil, err
	}

	ingestion, err := NewIngestionModule(platform)
	if err != nil {
		return nil, err
	}

	return &IngestionApplication{
		IngestionModule: ingestion,
		Platform:        platform,
		Baseline:        baselineModule,
	}, nil
}

// NewIngestionModule migra el esquema de Ingestion y cablea sus servicios.
func NewIngestionModule(platform *Platform) (*IngestionModule, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logIngestion := logger.New("Ingestion", "Info")

	db := platform.DB

	// ---------------------------------------------------------
	// Base de datos
	// ---------------------------------------------------------

	if err := migrateIngestionSchema(db, logDB); err != nil {
		return nil, err
	}

	// ---------------------------------------------------------
	// Clientes DataOS
	// ---------------------------------------------------------

	username := os.Getenv("DATAOS_USERNAME")
	password := os.Getenv("DATAOS_PASSWORD")

	if username == "" || password == "" {
		logIngestion.Error("faltan las credenciales de DataOS", "variables", "DATAOS_USERNAME, DATAOS_PASSWORD")

		return nil, fmt.Errorf("credenciales de DataOS no configuradas")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("crear cookie jar: %w", err)
	}

	httpClient := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	location, err := loadIngestionLocation()
	if err != nil {
		return nil, err
	}

	dataOSClient := dataos.NewClient(
		auth.NewClient(httpClient, dataOSBaseURL, username, password),
		dashboard.NewClient(httpClient, dataOSBaseURL),
		mapping.NewSeriesExtractor(logIngestion),
		mapping.NewMapper(mapping.NewTimestampParser(location)),
		logIngestion,
	)

	// ---------------------------------------------------------
	// Servicio
	// ---------------------------------------------------------

	devices := repositories.NewDeviceRepository(db)
	ingestService := services.NewIngestService(
		dataOSClient,
		repositories.NewIngestionStateRepository(db),
		devices,
		repositories.NewUnitOfWork(db, logIngestion),
		logIngestion,
		services.NewIngestionValidator(logIngestion),
	)
	coverage := repositories.NewMeasurementCoverageRepository(db)

	return &IngestionModule{
		IngestService: ingestService,
		CatchUpService: func(cfg services.CatchUpConfig) *services.CatchUpService {
			return services.NewCatchUpService(devices, coverage, ingestService, cfg, location, logIngestion)
		},
		Logger: logIngestion,
	}, nil
}

// loadIngestionLocation carga ingestionTimezone.
func loadIngestionLocation() (*time.Location, error) {
	location, err := time.LoadLocation(ingestionTimezone)
	if err != nil {
		return nil, fmt.Errorf("cargar la zona horaria %s: %w", ingestionTimezone, err)
	}
	return location, nil
}

// migrateIngestionSchema crea/actualiza las tablas de Ingestion. La comparten
// la ingesta por DataOS y el callback de Dataflow Labs. MetricDefinitionModel se
// conserva solo para mantener su tabla (el Dashboard la lee).
func migrateIngestionSchema(db *gorm.DB, logDB *logger.Logger) error {
	if err := db.AutoMigrate(
		&models.DeviceModel{},
		&models.MetricDefinitionModel{},
		&models.RawMeasurementModel{},
		&models.IngestionStateModel{},
	); err != nil {
		logDB.Error("falló la migración de Ingestion", "error", err)
		return err
	}
	return nil
}
