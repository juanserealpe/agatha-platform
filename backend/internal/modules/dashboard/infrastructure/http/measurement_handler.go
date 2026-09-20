package http

import (
	"net/http"
	"strconv"
	"time"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
)

type MeasurementHandler struct {
	dashboardService *services.DashboardService
	dashboardLogger  *logger.Logger
}

func NewMeasurementHandler(dashboardService *services.DashboardService,
	dashboardLogger *logger.Logger) *MeasurementHandler {
	return &MeasurementHandler{
		dashboardService: dashboardService,
		dashboardLogger:  dashboardLogger,
	}
}

func (h *MeasurementHandler) GetMeasurements(w http.ResponseWriter, r *http.Request) {

	h.dashboardLogger.Info("[ENDPOINT/MEASUREMENT] GET - Iniciando procesamiento de solicitud")

	query := r.URL.Query()

	deviceID, err := strconv.ParseInt(query.Get("deviceId"), 10, 64)
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - Error al parsear deviceId")
		sharedhttp.HandleError(w, appErrors.ErrInvalidDate)
		return
	}

	startDate, err := time.Parse(time.RFC3339, query.Get("startDate"))
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - Error al parsear startDate")
		sharedhttp.HandleError(w, appErrors.ErrInvalidDate)
		return
	}

	endDate, err := time.Parse(time.RFC3339, query.Get("endDate"))
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - Error al parsear endDate")
		sharedhttp.HandleError(w, appErrors.ErrInvalidDate)
		return
	}

	response, err := h.dashboardService.GetMeasurements(r.Context(), deviceID, startDate, endDate)
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - Error al obtener mediciones")
		sharedhttp.HandleError(w, err)
		return
	}
	h.dashboardLogger.Info("[ENDPOINT/MEASUREMENT] GET - Mediciones obtenidas correctamente")
	sharedhttp.WriteJSON(w, http.StatusOK, response)
}
