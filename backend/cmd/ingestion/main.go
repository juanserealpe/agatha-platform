package main

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/mapping"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const dataOSBaseURL = "https://app.dataos.dataflowlabs.io"

func main() {

	// ---------------------------------------------------------
	// Contexto
	// ---------------------------------------------------------

	ctx := context.Background()

	// ---------------------------------------------------------
	// Logger
	// ---------------------------------------------------------

	log := logger.New("INFO")

	// ---------------------------------------------------------
	// Credenciales
	// ---------------------------------------------------------

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

	// ---------------------------------------------------------
	// Servicio de Ingesta
	// ---------------------------------------------------------

	service := services.NewIngestService(
		dataosClient,
		mapper,
		seriesExtractor,
		nil,
		nil,
		nil,
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
}
