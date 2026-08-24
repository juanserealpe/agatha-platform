package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
	"gorm.io/gorm"
)

const dataOSBaseURL = "https://app.dataos.dataflowlabs.io"

func main() {

	// ---------------------------------------------------------
	// Contexto
	// ---------------------------------------------------------

	//ctx := context.Background()

	// ---------------------------------------------------------
	// Logger
	// ---------------------------------------------------------

	log := logger.New("INFO")

	// ---------------------------------------------------------
	// Base de Datos
	// ---------------------------------------------------------
	if err := godotenv.Load(); err != nil {
		log.Error("No .env file found, using environment variables")
	}

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		log.Error("DATABASE_URL is not configured")
	}

	db, err := database.NewPostgres(dsn)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
	}

	_ = db

	//Migración de la base de datos

	err = db.AutoMigrate(
		&models.DeviceModel{},
		&models.MetricDefinitionModel{},
		&models.RawMeasurementModel{},
	)

	if err != nil {
		log.Error("failed to migrate database", "error", err)
		return
	}

	if err := seedMetricDefinitions(db, log); err != nil {
		log.Error("failed to seed metric definitions", "error", err)
		return
	}

	devices := []models.DeviceModel{
		{
			ID:               "11556",
			MeasuringPointID: 11556,
			Name:             "Agatha 02",
			Active:           true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}

	if err := db.Create(&devices).Error; err != nil {
		log.Error("failed to seed devices", "error", err)
	}

	log.Info("Devices seeded successfully")

	// ---------------------------------------------------------
	// Credenciales
	// ---------------------------------------------------------
	/*
		username := os.Getenv("DATAOS_USERNAME")
		password := os.Getenv("DATAOS_PASSWORD")

		if username == "" || password == "" {
			log.Error(
				"las credenciales de DataOS no están configuradas",
				"required_variables", "DATAOS_USERNAME, DATAOS_PASSWORD",
			)

			os.Exit(1)
		}

		// ---------------------------------------------------------
		// Cliente HTTP + Cookie Jar
		// ---------------------------------------------------------

		jar, err := cookiejar.New(nil)
		if err != nil {
			log.Error(
				"falló la creación del contenedor de cookies HTTP",
				"error", err,
			)

			os.Exit(1)
		}

		httpClient := &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		}

		// ---------------------------------------------------------
		// Cliente de Autenticación DataOS
		// ---------------------------------------------------------

		authClient := auth.NewClient(
			httpClient,
			dataOSBaseURL,
			username,
			password,
		)

		// ---------------------------------------------------------
		// Cliente de Dashboard DataOS
		// ---------------------------------------------------------

		dashboardClient := dashboard.NewClient(
			httpClient,
			dataOSBaseURL,
		)

		// ---------------------------------------------------------
		// Fachada DataOS
		// ---------------------------------------------------------

		dataosClient := dataos.NewClient(
			authClient,
			dashboardClient,
		)

		// ---------------------------------------------------------
		// Catálogo de Métricas
		// ---------------------------------------------------------

		metricCatalog := mapping.NewMetricCatalog()

		// ---------------------------------------------------------
		// Analizador de Timestamps
		// ---------------------------------------------------------

		timestampParser, err := mapping.NewTimestampParser()
		if err != nil {
			log.Error(
				"falló la creación del analizador de timestamps",
				"error", err,
			)

			os.Exit(1)
		}

		// ---------------------------------------------------------
		// Mapeador
		// ---------------------------------------------------------

		mapper := mapping.NewMapper(
			metricCatalog,
			timestampParser,
		)

		// ---------------------------------------------------------
		// Extractor de Series
		// ---------------------------------------------------------

		seriesExtractor := mapping.NewSeriesExtractor(
			log,
		)

		// ---------------------------------------------------------
		// Repositorios
		//
		// TEMPORAL:
		// Todavía no usamos persistencia.
		// ---------------------------------------------------------
		deviceRepository := repositories.NewDeviceRepository(db)

		metricDefinitionRepository :=
			repositories.NewMetricDefinitionRepository(db)

		measurementRepository :=
			repositories.NewMeasurementRepository(db)
		// ---------------------------------------------------------
		// Servicio de Ingesta
		// ---------------------------------------------------------

		service := services.NewIngestService(
			dataosClient,
			mapper,
			seriesExtractor,
			deviceRepository,
			metricDefinitionRepository,
			measurementRepository,
			log,
		)

		// ---------------------------------------------------------
		// Dispositivo temporal para pruebas
		// ---------------------------------------------------------

		device := &entities.Device{
			ID:               "test-device",
			MeasuringPointID: 11556,
			Name:             "DataOS Test Device",
			Active:           true,
		}

		// ---------------------------------------------------------
		// Período de prueba
		// ---------------------------------------------------------

		end := time.Now()

		start := end.Add(-1 * time.Hour)

		log.Info(
			"iniciando prueba de ingesta",
			"measuring_point_id", device.MeasuringPointID,
			"start", start,
			"end", end,
		)

		// ---------------------------------------------------------
		// Ejecutar Ingesta
		// ---------------------------------------------------------

		if err := service.Ingest(
			ctx,
			device,
			start,
			end,
		); err != nil {

			log.Error(
				"falló la prueba de ingesta",
				"error", err,
			)

			os.Exit(1)
		}

		// ---------------------------------------------------------
		// Finalizado
		// ---------------------------------------------------------

		log.Info(
			"prueba de ingesta completada exitosamente",
		)
	*/
}

func seedMetricDefinitions(
	db *gorm.DB,
	log *logger.Logger,
) error {

	metrics := []models.MetricDefinitionModel{
		// Sensors
		{
			Key:      "pitch",
			Name:     "Pitch",
			Category: string(enums.CategorySensor),
			Active:   true,
		},
		{
			Key:      "roll",
			Name:     "Roll",
			Category: string(enums.CategorySensor),
			Active:   true,
		},
		{
			Key:      "voltaje",
			Name:     "Voltaje",
			Category: string(enums.CategorySensor),
			Active:   true,
		},
		{
			Key:      "corriente",
			Name:     "Corriente",
			Category: string(enums.CategorySensor),
			Active:   true,
		},
		{
			Key:      "bateria",
			Name:     "Batería",
			Category: string(enums.CategorySensor),
			Active:   true,
		},

		// Features
		{
			Key:      "rms",
			Name:     "RMS",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "centroide_espectral",
			Name:     "Centroide espectral",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "planitud_espectral",
			Name:     "Planitud espectral",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "deriva",
			Name:     "Deriva",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "actividad",
			Name:     "Actividad",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "mag_promedio",
			Name:     "Indicadores de fuga acustica",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "act_pico",
			Name:     "Act. pico",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "razib_r",
			Name:     "Razib R",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "b0_viento_30_300hz",
			Name:     "B0 viento 30-300Hz",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "b1_fuga_500_1200hz",
			Name:     "B1 fuga 500-1200Hz",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "b2_fuga_1200_2000hz",
			Name:     "B2 fuga 1200-2000Hz",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "b3_guarda_2_2_4khz",
			Name:     "B3 guarda 2-2.4kHz",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "pitch_base",
			Name:     "Pitch base",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "roll_base",
			Name:     "Roll base",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},
		{
			Key:      "rms_acustico",
			Name:     "RMS acustico",
			Category: string(enums.CategoryFeature),
			Active:   true,
		},

		// Status
		{
			Key:      "fuga_sospechada",
			Name:     "Fuga sospechada",
			Category: string(enums.CategoryStatus),
			Active:   true,
		},
	}

	for _, metric := range metrics {

		var existing models.MetricDefinitionModel

		err := db.
			Where("key = ?", metric.Key).
			First(&existing).
			Error

		if err == nil {
			log.Info(
				"metric definition already exists",
				"key", metric.Key,
				"id", existing.ID,
			)
			continue
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf(
				"check metric %q: %w",
				metric.Key,
				err,
			)
		}

		if err := db.Create(&metric).Error; err != nil {
			return fmt.Errorf(
				"create metric %q: %w",
				metric.Key,
				err,
			)
		}

		log.Info(
			"metric definition created",
			"key", metric.Key,
			"id", metric.ID,
		)
	}

	return nil
}
