package bootstrap

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/mapping"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const dataOSBaseURL = "https://app.dataos.dataflowlabs.io"

type IngestionApplication struct {
	IngestService *services.IngestService
	Logger        *logger.Logger
}

func NewIngestionApplication() (*IngestionApplication, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logAuthorization := logger.New("Authorization", "Info")
	logIngestion := logger.New("Ingestion", "Info")

	// ---------------------------------------------------------
	// Variables de entorno
	// ---------------------------------------------------------

	if err := godotenv.Load(); err != nil {
		logDB.Error("no se encontró archivo .env; se utilizarán variables de entorno")
	}

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		logDB.Error("DATABASE_URL no está configurada")
		return nil, fmt.Errorf("DATABASE_URL no está configurada")
	}

	// ---------------------------------------------------------
	// Base de datos
	// ---------------------------------------------------------

	db, err := database.NewPostgres(dsn)
	if err != nil {
		logDB.Error("falló la conexión con la base de datos", "error", err)
		return nil, err
	}

	if err := db.AutoMigrate(
		&models.DeviceModel{},
		&models.MetricDefinitionModel{},
		&models.RawMeasurementModel{},
		&models.IngestionStateModel{},
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
	)

	return &IngestionApplication{
		IngestService: ingestService,
		Logger:        logIngestion,
	}, nil
}
