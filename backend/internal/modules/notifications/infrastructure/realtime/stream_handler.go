package realtime

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// RouteStream es el stream SSE de alertas (canal web).
const RouteStream = "GET /api/notifications/stream"

// LastEventIDParam es la alternativa a la cabecera Last-Event-ID para la
// reconexión manual del cliente (un EventSource nuevo no la envía).
const LastEventIDParam = "last_event_id"

// reconnectDelay es el retry: que se le sugiere al EventSource del navegador.
const reconnectDelay = 3 * time.Second

// StreamHandler expone el hub como text/event-stream.
type StreamHandler struct {
	hub *Hub
	log *logger.Logger
}

func NewStreamHandler(hub *Hub, log *logger.Logger) *StreamHandler {
	return &StreamHandler{hub: hub, log: log}
}

func (h *StreamHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteStream, h.Stream)
}

// Stream mantiene la conexión abierta y escribe los eventos del hub hasta que
// el cliente se va, el hub lo suelta (lento o apagado) o falla la escritura.
func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get(LastEventIDParam)
	}

	client, pending, err := h.hub.Connect(lastEventID)
	switch {
	case errors.Is(err, ErrTooManyClients), errors.Is(err, ErrHubClosed):
		w.Header().Set("Retry-After", "30")
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer h.hub.Disconnect(client)

	// El stream no tiene plazo. Con Go 1.25 el ReadTimeout de cmd/platform ya
	// no lo corta, pero un WriteTimeout sí lo haría: se limpian los dos.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no") // nginx no debe acumular el stream
	w.WriteHeader(http.StatusOK)

	write := func(b []byte) bool {
		if _, err := w.Write(b); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	if !write([]byte("retry: " + strconv.FormatInt(reconnectDelay.Milliseconds(), 10) + "\n\n")) {
		h.log.Error("[StreamHandler] La respuesta no admite streaming")
		return
	}
	for _, f := range pending {
		if !write(f.Bytes()) {
			return
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case f, ok := <-client.Frames():
			if !ok || !write(f.Bytes()) {
				return
			}
		}
	}
}
