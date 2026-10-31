package http

import (
	"encoding/json"
	"net/http"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// RouteTestEvent publica un evento de alerta inventado. Solo se registra fuera
// de producción y con NOTIFICATIONS_TEST_ENDPOINT=true (ver bootstrap).
const RouteTestEvent = "POST /api/notifications/test-event"

const maxTestEventBodyBytes = 4 << 10

// TestEventRequest es el cuerpo. Estricto (campos desconocidos ⇒ 400): es una
// herramienta de desarrollo y así se ven los errores de tipeo.
type TestEventRequest struct {
	MeasuringPointID int64  `json:"measuring_point_id"`
	AlertID          int64  `json:"alert_id"`
	EventType        string `json:"event_type"`
	Level            string `json:"level"`
	Code             string `json:"code"`
	Change           string `json:"change"`
	Classification   string `json:"classification"`
	Late             bool   `json:"late"`
}

// TestEventResponse dice qué se publicó.
type TestEventResponse struct {
	AlertID   int64  `json:"alert_id"`
	AlertCode string `json:"alert_code"`
	MessageID *int64 `json:"message_id"`
	Change    string `json:"change"`
}

// TestEventHandler expone POST /api/notifications/test-event.
type TestEventHandler struct {
	emitter ports.TestEventEmitter
	log     *logger.Logger
}

func NewTestEventHandler(emitter ports.TestEventEmitter, log *logger.Logger) *TestEventHandler {
	return &TestEventHandler{emitter: emitter, log: log}
}

func (h *TestEventHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteTestEvent, h.Emit)
}

// Emit valida y publica el evento: responde 202 porque la entrega a cada canal
// es asíncrona (colas del bus).
func (h *TestEventHandler) Emit(w http.ResponseWriter, r *http.Request) {
	var req TestEventRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTestEventBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.More() {
		h.reject(w, "el cuerpo debe ser un JSON de hasta 4 KB, sin campos desconocidos")
		return
	}
	event, err := entities.NewTestEvent(entities.TestEvent{
		MeasuringPointID: req.MeasuringPointID, AlertID: req.AlertID, EventType: req.EventType, Level: req.Level,
		Code: req.Code, Change: req.Change, Classification: req.Classification, Late: req.Late,
	})
	if err != nil {
		h.reject(w, err.Error())
		return
	}

	result, err := h.emitter.Emit(r.Context(), event)
	if err != nil {
		h.log.Error("[ENDPOINT/NOTIFICATIONS] POST test-event - No se pudo publicar", "error", err)
		sharedhttp.HandleError(w, err)
		return
	}
	h.log.Warn("[ENDPOINT/NOTIFICATIONS] POST test-event - Evento de prueba publicado",
		"alert", result.AlertCode, "change", result.Change, "message_id", result.MessageID, "level", event.Level)

	resp := TestEventResponse{AlertID: result.AlertID, AlertCode: result.AlertCode, Change: result.Change}
	if result.MessageID != 0 {
		resp.MessageID = &result.MessageID
	}
	sharedhttp.WriteJSON(w, http.StatusAccepted, resp)
}

func (h *TestEventHandler) reject(w http.ResponseWriter, message string) {
	h.log.Warn("[ENDPOINT/NOTIFICATIONS] POST test-event - Solicitud rechazada", "reason", message)
	sharedhttp.HandleError(w, &appErrors.AppError{Code: appErrors.CodeInvalidInput, Message: message})
}
