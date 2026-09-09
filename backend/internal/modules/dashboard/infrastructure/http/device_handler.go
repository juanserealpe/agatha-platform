package http

import (
	"encoding/json"
	"net/http"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
)

type DeviceHandler struct {
	dashboardService *services.DashboardService
}

func NewDeviceHandler(
	dashboardService *services.DashboardService,
) *DeviceHandler {
	return &DeviceHandler{
		dashboardService: dashboardService,
	}
}

func (h *DeviceHandler) GetAllDevices(
	w http.ResponseWriter,
	r *http.Request,
) {
	devices, err := h.dashboardService.GetAllDevices(r.Context())
	if err != nil {
		http.Error(
			w,
			"Error getting devices",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(devices); err != nil {
		http.Error(
			w,
			"Error encoding response",
			http.StatusInternalServerError,
		)
		return
	}
}
