package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
)

type SeriesHandler struct {
	seriesService   *services.SeriesService
	dashboardLogger *logger.Logger
}

func NewSeriesHandler(seriesService *services.SeriesService, dashboardLogger *logger.Logger) *SeriesHandler {
	return &SeriesHandler{
		seriesService:   seriesService,
		dashboardLogger: dashboardLogger,
	}
}

// GetVariables devuelve el catálogo de variables graficables.
func (h *SeriesHandler) GetVariables(w http.ResponseWriter, _ *http.Request) {
	sharedhttp.WriteJSON(w, http.StatusOK, h.seriesService.GetVariables())
}

// GetSeries devuelve las variables pedidas de un dispositivo en un rango.
// Query: deviceId, startDate, endDate (RFC3339) y, opcional, variables
// (claves separadas por coma; sin el parámetro devuelve todas).
func (h *SeriesHandler) GetSeries(w http.ResponseWriter, r *http.Request) {
	h.dashboardLogger.Info("[ENDPOINT/SERIES] GET - Iniciando procesamiento de solicitud")

	query := r.URL.Query()

	deviceIDParam := query.Get("deviceId")
	if deviceIDParam == "" {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - deviceId ausente")
		sharedhttp.HandleError(w, domain.ErrMissingDeviceID)
		return
	}

	deviceID, err := strconv.ParseInt(deviceIDParam, 10, 64)
	if err != nil || deviceID <= 0 {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - deviceId inválido", "deviceId", deviceIDParam)
		sharedhttp.HandleError(w, domain.ErrInvalidDeviceID)
		return
	}

	startDateParam := query.Get("startDate")
	if startDateParam == "" {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - startDate ausente")
		sharedhttp.HandleError(w, domain.ErrMissingStartDate)
		return
	}

	startDate, err := time.Parse(time.RFC3339, startDateParam)
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - startDate inválida", "startDate", startDateParam)
		sharedhttp.HandleError(w, domain.ErrInvalidStartDate)
		return
	}

	endDateParam := query.Get("endDate")
	if endDateParam == "" {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - endDate ausente")
		sharedhttp.HandleError(w, domain.ErrMissingEndDate)
		return
	}

	endDate, err := time.Parse(time.RFC3339, endDateParam)
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - endDate inválida", "endDate", endDateParam)
		sharedhttp.HandleError(w, domain.ErrInvalidEndDate)
		return
	}

	response, err := h.seriesService.GetSeries(r.Context(), deviceID, startDate, endDate, parseVariables(query.Get("variables")))
	if err != nil {
		h.dashboardLogger.Error("[ENDPOINT/SERIES] GET - Error al obtener las series")
		sharedhttp.HandleError(w, err)
		return
	}

	h.dashboardLogger.Info("[ENDPOINT/SERIES] GET - Series obtenidas correctamente")
	sharedhttp.WriteJSON(w, http.StatusOK, response)
}

// parseVariables separa "b1, B2,,indicador" en ["b1", "b2", "indicador"].
func parseVariables(raw string) []string {
	keys := []string{}
	for _, part := range strings.Split(raw, ",") {
		if key := strings.ToLower(strings.TrimSpace(part)); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}
