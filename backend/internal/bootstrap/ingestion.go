package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

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
	location, err := time.LoadLocation("America/Bogota")
	if err != nil {
		return nil, fmt.Errorf("cargar ubicación: %w", err)
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
// cmd/scheduler): Platform + Baseline + Ingestion + Processing en el mismo bus.
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

// NewIngestionApplication compone la Platform con Baseline, Ingestion y Processing.
// Processing se registra porque el bus es en proceso: sin él los eventos se descartan.
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

	NewProcessingModule(platform)

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
	logAuthorization := logger.New("Authorization", "Info")
	logIngestion := logger.New("Ingestion", "Info")

	db := platform.DB

	// ---------------------------------------------------------
	// Base de datos
	// ---------------------------------------------------------

	if err := db.AutoMigrate(
		&models.DeviceModel{},
		&models.MetricDefinitionModel{},
		&models.RawMeasurementModel{},
		&models.IngestionStateModel{},
		&models.IngestionGapModel{},
	); err != nil {
		logDB.Error("falló la migración de la base de datos", "error", err)
		return nil, err
	}

	// ---------------------------------------------------------
	// Clientes DataOS
	// ---------------------------------------------------------

	username := os.Getenv("DATAOS_USERNAME")
	password := os.Getenv("DATAOS_PASSWORD")

	if username == "" || password == "" {
		logAuthorization.Error(
			"las credenciales de DataOS no están configuradas",
			"required_variables", "DATAOS_USERNAME, DATAOS_PASSWORD",
		)

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

	authClient := auth.NewClient(httpClient, dataOSBaseURL, username, password)
	dashboardClient := dashboard.NewClient(httpClient, dataOSBaseURL)
	seriesExtractor := mapping.NewSeriesExtractor(logIngestion)

	location, err := time.LoadLocation("America/Bogota")
	if err != nil {
		return nil, fmt.Errorf("cargar ubicación: %w", err)
	}

	timestampParser := mapping.NewTimestampParser(location)
	mapper := mapping.NewMapper(timestampParser)

	dataOSClient := dataos.NewClient(
		authClient,
		dashboardClient,
		seriesExtractor,
		mapper,
		logIngestion,
	)

	// ---------------------------------------------------------
	// Repositories
	// ---------------------------------------------------------

	deviceRepository := repositories.NewDeviceRepository(db)
	ingestionStateRepository := repositories.NewIngestionStateRepository(db)

	// ---------------------------------------------------------
	// Services
	// ---------------------------------------------------------

	processingService := services.NewProcessingService(logIngestion)
	ingestionValidator := services.NewIngestionValidator(logIngestion)

	// ---------------------------------------------------------
	// Unit of Work
	// ---------------------------------------------------------

	unitOfWork := repositories.NewUnitOfWork(db, logIngestion)

	// ---------------------------------------------------------
	// Event Publisher
	// ---------------------------------------------------------

	eventPublisher := events.NewBusEventPublisher(platform.EventBus)

	// ---------------------------------------------------------
	// Ingestion Service
	// ---------------------------------------------------------

	ingestService := services.NewIngestService(
		dataOSClient,
		ingestionStateRepository,
		deviceRepository,
		processingService,
		unitOfWork,
		logIngestion,
		ingestionValidator,
		eventPublisher,
	)

	return &IngestionModule{
		IngestService: ingestService,
		Logger:        logIngestion,
	}, nil
}
