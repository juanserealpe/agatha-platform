package bootstrap

import (
	"net/http"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	dashboardEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	dashboardPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	dashboardHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DashboardModule expone los handlers HTTP de Dashboard (solo lectura de la BD).
type DashboardModule struct {
	deviceHandler      *dashboardHTTP.DeviceHandler
	measurementHandler *dashboardHTTP.MeasurementHandler
	seriesHandler      *dashboardHTTP.SeriesHandler
}

// NewDashboardModule cablea repositorios, servicio y handlers de Dashboard.
func NewDashboardModule(platform *Platform) *DashboardModule {
	logDashboard := logger.New("Dashboard", "Info")

	deviceRepository := repositories.NewDeviceRepository(platform.DB)
	measurementRepository := repositories.NewMeasurementRepository(platform.DB)

	dashboardValidator := services.NewDashboardValidator(logDashboard)
	dashboardService := services.NewDashboardService(
		deviceRepository,
		measurementRepository,
		dashboardValidator,
		logDashboard,
	)

	// Un lector por fuente de variables graficables. Las variables en sí viven
	// en el catálogo del módulo (entities.SeriesCatalog).
	seriesService := services.NewSeriesService(
		deviceRepository,
		map[dashboardEntities.SeriesSource]dashboardPorts.SeriesReader{
			dashboardEntities.SourceMeasurement: repositories.NewMeasurementSeriesReader(platform.DB),
			dashboardEntities.SourceEvaluation:  repositories.NewEvaluationSeriesReader(platform.DB),
		},
		dashboardValidator,
		logDashboard,
	)

	return &DashboardModule{
		deviceHandler:      dashboardHTTP.NewDeviceHandler(dashboardService, logDashboard),
		measurementHandler: dashboardHTTP.NewMeasurementHandler(dashboardService, logDashboard),
		seriesHandler:      dashboardHTTP.NewSeriesHandler(seriesService, logDashboard),
	}
}

// RegisterRoutes registra los endpoints de Dashboard en el mux.
func (m *DashboardModule) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard/devices", m.deviceHandler.GetAllDevices)
	mux.HandleFunc("GET /api/dashboard/measurements", m.measurementHandler.GetMeasurements)
	mux.HandleFunc("GET /api/dashboard/variables", m.seriesHandler.GetVariables)
	mux.HandleFunc("GET /api/dashboard/series", m.seriesHandler.GetSeries)
}
