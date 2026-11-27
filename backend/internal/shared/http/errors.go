package http

import (
	"errors"
	"net/http"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

var errorStatus = map[appErrors.ErrorCode]int{
	appErrors.CodeNotFound:                http.StatusNotFound,
	appErrors.CodeInvalidInput:            http.StatusBadRequest,
	appErrors.CodeConflict:                http.StatusConflict,
	appErrors.CodeFutureDate:              http.StatusBadRequest,
	appErrors.CodeAlreadyIngested:         http.StatusConflict,
	appErrors.CodeBeforeInitialDate:       http.StatusBadRequest,
	appErrors.CodeInvalidMeasuringPointID: http.StatusBadRequest,

	appErrors.CodeConsultingLastState:    http.StatusInternalServerError,
	appErrors.CodeGettingIngestedData:    http.StatusBadGateway,
	appErrors.CodeConsultingDevices:      http.StatusInternalServerError,
	appErrors.CodeConsultingMeasurements: http.StatusInternalServerError,
	appErrors.CodeConsultingBaseline:     http.StatusInternalServerError,
	appErrors.CodePersistingBaseline:     http.StatusInternalServerError,
	appErrors.CodeConsultingEvaluations:  http.StatusInternalServerError,
	appErrors.CodePersistingEvaluations:  http.StatusInternalServerError,

	appErrors.CodeConsultingThresholds:      http.StatusInternalServerError,
	appErrors.CodePersistingThresholds:      http.StatusInternalServerError,
	appErrors.CodeConsultingRuleEvaluations: http.StatusInternalServerError,
	appErrors.CodePersistingRuleEvaluations: http.StatusInternalServerError,
	appErrors.CodeConsultingAlerts:          http.StatusInternalServerError,
	appErrors.CodePersistingAlerts:          http.StatusInternalServerError,

	appErrors.CodeUnauthorized:       http.StatusUnauthorized,
	appErrors.CodeForbidden:          http.StatusForbidden,
	appErrors.CodeInvalidCredentials: http.StatusUnauthorized,
	appErrors.CodeTooManyAttempts:    http.StatusTooManyRequests,
	appErrors.CodeAuthNotConfigured:  http.StatusServiceUnavailable,
}

func HandleError(w http.ResponseWriter, err error) {
	var appErr *appErrors.AppError

	if errors.As(err, &appErr) {
		status, exists := errorStatus[appErr.Code]

		if !exists {
			status = http.StatusInternalServerError
		}

		WriteJSON(w, status, map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
		})

		return
	}

	WriteJSON(w, http.StatusInternalServerError, map[string]string{
		"code":    "INTERNAL_ERROR",
		"message": "error interno del servidor",
	})
}
