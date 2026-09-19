package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
)

type MeasurementHandler struct {
	dashboardService *services.DashboardService
}

func NewMeasurementHandler(
	dashboardService *services.DashboardService,
) *MeasurementHandler {

	return &MeasurementHandler{
		dashboardService: dashboardService,
	}
}

func (h *MeasurementHandler) GetMeasurements(
	w http.ResponseWriter,
	r *http.Request,
) {

	// ==========================================
	// 1. Obtener deviceId
	// ==========================================

	deviceIDParam := r.URL.Query().Get("deviceId")

	if deviceIDParam == "" {
		http.Error(
			w,
			"deviceId is required",
			http.StatusBadRequest,
		)
		return
	}

	deviceID, err := strconv.ParseInt(
		deviceIDParam,
		10,
		64,
	)

	if err != nil || deviceID <= 0 {
		http.Error(
			w,
			"deviceId must be a valid positive integer",
			http.StatusBadRequest,
		)
		return
	}

	// ==========================================
	// 2. Obtener startDate
	// ==========================================

	startDateParam := r.URL.Query().Get("startDate")

	if startDateParam == "" {
		http.Error(
			w,
			"startDate is required",
			http.StatusBadRequest,
		)
		return
	}

	startDate, err := time.Parse(
		time.RFC3339,
		startDateParam,
	)

	if err != nil {
		http.Error(
			w,
			"startDate must be a valid RFC3339 date",
			http.StatusBadRequest,
		)
		return
	}

	// ==========================================
	// 3. Obtener endDate
	// ==========================================

	endDateParam := r.URL.Query().Get("endDate")

	if endDateParam == "" {
		http.Error(
			w,
			"endDate is required",
			http.StatusBadRequest,
		)
		return
	}

	endDate, err := time.Parse(
		time.RFC3339,
		endDateParam,
	)

	if err != nil {
		http.Error(
			w,
			"endDate must be a valid RFC3339 date",
			http.StatusBadRequest,
		)
		return
	}

	// ==========================================
	// 4. Validar rango
	// ==========================================

	if !startDate.Before(endDate) {
		http.Error(
			w,
			"startDate must be before endDate",
			http.StatusBadRequest,
		)
		return
	}

	// ==========================================
	// 5. Consultar Service
	// ==========================================

	response, err := h.dashboardService.GetMeasurements(
		r.Context(),
		deviceID,
		startDate,
		endDate,
	)

	if err != nil {
		http.Error(
			w,
			"failed to get measurements",
			http.StatusInternalServerError,
		)
		return
	}

	// ==========================================
	// 6. Responder JSON
	// ==========================================

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(
			w,
			"failed to encode response",
			http.StatusInternalServerError,
		)
		return
	}
}
