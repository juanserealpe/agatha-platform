package http

import (
	"net/http"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type DeviceHandler struct {
	dashboardService *services.DashboardService
	dashboardLogger  *logger.Logger
}

func NewDeviceHandler(
	dashboardService *services.DashboardService,
	dashboardLogger *logger.Logger,
) *DeviceHandler {
	return &DeviceHandler{
		dashboardService: dashboardService,
		dashboardLogger:  dashboardLogger,
	}
}

func (h *DeviceHandler) GetAllDevices(w http.ResponseWriter, r *http.Request,
) {
	h.dashboardLogger.Info("[ENDPOINT/DEVICE] GET - Iniciando procesamiento de solicitud")
	devices, err := h.dashboardService.GetAllDevices(r.Context())
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/DEVICE] GET - Error al obtener dispositivos")
		sharedhttp.HandleError(w, err)
		return
	}
	h.dashboardLogger.Info("[ENDPOINT/DEVICE] GET - Dispositivos obtenidos correctamente")
	sharedhttp.WriteJSON(w, http.StatusOK, devices)
}
