package http

import (
	"net/http"
	"strconv"
	"time"

	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
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

	deviceIDParam := query.Get("deviceId")
	if deviceIDParam == "" {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - deviceId ausente")
		sharedhttp.HandleError(w, domain.ErrMissingDeviceID)
		return
	}

	deviceID, err := strconv.ParseInt(deviceIDParam, 10, 64)
	if err != nil || deviceID <= 0 {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - deviceId inválido", "deviceId", deviceIDParam)
		sharedhttp.HandleError(w, domain.ErrInvalidDeviceID)
		return
	}

	startDateParam := query.Get("startDate")
	if startDateParam == "" {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - startDate ausente")
		sharedhttp.HandleError(w, domain.ErrMissingStartDate)
		return
	}

	startDate, err := time.Parse(time.RFC3339, startDateParam)
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - startDate inválida", "startDate", startDateParam)
		sharedhttp.HandleError(w, domain.ErrInvalidStartDate)
		return
	}

	endDateParam := query.Get("endDate")
	if endDateParam == "" {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - endDate ausente")
		sharedhttp.HandleError(w, domain.ErrMissingEndDate)
		return
	}

	endDate, err := time.Parse(time.RFC3339, endDateParam)
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/MEASUREMENT] GET - endDate inválida", "endDate", endDateParam)
		sharedhttp.HandleError(w, domain.ErrInvalidEndDate)
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
