package bootstrap

import (
	"net/http"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	dashboardHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DashboardModule expone los handlers HTTP de Dashboard (solo lectura de la BD).
type DashboardModule struct {
	deviceHandler      *dashboardHTTP.DeviceHandler
	measurementHandler *dashboardHTTP.MeasurementHandler
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

	return &DashboardModule{
		deviceHandler:      dashboardHTTP.NewDeviceHandler(dashboardService, logDashboard),
		measurementHandler: dashboardHTTP.NewMeasurementHandler(dashboardService, logDashboard),
	}
}

// RegisterRoutes registra los endpoints de Dashboard en el mux.
func (m *DashboardModule) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard/devices", m.deviceHandler.GetAllDevices)
	mux.HandleFunc("GET /api/dashboard/measurements", m.measurementHandler.GetMeasurements)
}
