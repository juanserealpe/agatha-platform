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
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/events"
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

// Configuración de la ingesta programada.
const (
	scheduledMeasuringPointID int64 = 11556
	schedulerInterval               = 15 * time.Minute
	schedulerDelay                  = 50 * time.Second
)

// IngestionModule expone los servicios de Ingestion ya cableados.
type IngestionModule struct {
	IngestService *services.IngestService
	Logger        *logger.Logger
}

// NewScheduler crea el scheduler de ingesta periódica del módulo.
func (m *IngestionModule) NewScheduler() (*scheduler.IngestionScheduler, error) {
	location, err := loadIngestionLocation()
	if err != nil {
		return nil, err
	}

	return scheduler.NewIngestionScheduler(
		m.IngestService,
		m.Logger,
		scheduledMeasuringPointID,
		schedulerInterval,
		location,
		schedulerDelay,
	), nil
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

// NewIngestionModule migra el esquema de Ingestion y cablea sus servicios
// publicando eventos en el bus de la Platform. No conoce a sus consumidores.
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

	ingestService := services.NewIngestService(
		dataOSClient,
		repositories.NewIngestionStateRepository(db),
		repositories.NewDeviceRepository(db),
		repositories.NewUnitOfWork(db, logIngestion),
		logIngestion,
		services.NewIngestionValidator(logIngestion),
		events.NewBusEventPublisher(platform.EventBus),
	)

	return &IngestionModule{
		IngestService: ingestService,
		Logger:        logIngestion,
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
