// Package http expone los endpoints REST de Notifications. Sin autenticación
// (riesgo aceptado del piloto): los cuerpos tienen tope de tamaño.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// RoutePushToken registra el token FCM de un celular (docs/CONTRATO_PUSH_FCM.md
// §1). Va fuera de /api/alerts: Alert Management no expone push-devices.
const RoutePushToken = "PUT /api/devices/push-token"

// maxPushTokenBodyBytes cubre un token de 4096 y un deviceId de 128 con holgura.
const maxPushTokenBodyBytes = 4 << 10

var errInvalidBody = &appErrors.AppError{
	Code:    appErrors.CodeInvalidInput,
	Message: "el cuerpo debe ser un JSON de hasta 4 KB con deviceId, token y platform",
}

// PushTokens es el registro de tokens (en producción, *services.PushTokenService).
type PushTokens interface {
	Register(ctx context.Context, cmd services.RegisterPushTokenCommand) error
}

// PushTokenRequest es el cuerpo del registro. Los campos extra se ignoran: la
// app ya está publicada y no debe quedar reintentando por un campo nuevo.
type PushTokenRequest struct {
	DeviceID string `json:"deviceId"`
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// PushTokenHandler expone PUT /api/devices/push-token.
type PushTokenHandler struct {
	tokens PushTokens
	log    *logger.Logger
}

func NewPushTokenHandler(tokens PushTokens, log *logger.Logger) *PushTokenHandler {
	return &PushTokenHandler{tokens: tokens, log: log}
}

func (h *PushTokenHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RoutePushToken, h.Register)
}

// Register guarda el token y responde 204. Es idempotente: la app reintenta
// desde su cola de sincronización hasta recibir un 2xx.
func (h *PushTokenHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req PushTokenRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPushTokenBodyBytes))
	if err := decoder.Decode(&req); err != nil || decoder.More() {
		h.fail(w, errInvalidBody, "error", err)
		return
	}

	err := h.tokens.Register(r.Context(), services.RegisterPushTokenCommand{
		DeviceID: req.DeviceID, Token: req.Token, Platform: req.Platform,
	})
	switch {
	case errors.Is(err, entities.ErrInvalidPushDevice):
		h.fail(w, &appErrors.AppError{Code: appErrors.CodeInvalidInput, Message: err.Error()}, "device_id", req.DeviceID)
	case err != nil:
		h.log.Error("[ENDPOINT/NOTIFICATIONS] PUT push-token - No se pudo guardar el token", "device_id", req.DeviceID, "error", err)
		sharedhttp.HandleError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *PushTokenHandler) fail(w http.ResponseWriter, err *appErrors.AppError, fields ...any) {
	h.log.Warn("[ENDPOINT/NOTIFICATIONS] PUT push-token - Solicitud rechazada", append(fields, "reason", err.Message)...)
	sharedhttp.HandleError(w, err)
}
