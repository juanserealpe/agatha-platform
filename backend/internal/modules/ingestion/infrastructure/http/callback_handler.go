package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	sharedhttp "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const (
	// CallbackPath es la ruta que Dataflow Labs invoca (puerto 8080).
	CallbackPath = "/ingestion/callback"

	// MaxCallbackBodyBytes limita el cuerpo: un registro con decenas de variables
	// ocupa unos pocos KB.
	MaxCallbackBodyBytes = 1 << 20

	// callbackTimeout deja margen frente a los 10 s que espera Dataflow Labs.
	callbackTimeout = 8 * time.Second

	bearerPrefix = "bearer "
)

// callbackReceiver es el caso de uso que procesa un registro ya decodificado.
type callbackReceiver interface {
	Receive(ctx context.Context, request dtos.DataRecordCallback) (dtos.CallbackResponse, error)
}

// CallbackHandler recibe los registros de Dataflow Labs: autentica con Bearer
// (AGATHA_CALLBACK_TOKEN), valida método, Content-Type y tamaño, y decodifica.
type CallbackHandler struct {
	service      callbackReceiver
	tokenDigest  [sha256.Size]byte
	tokenEnabled bool
	logger       *logger.Logger
}

// NewCallbackHandler recibe el token configurado. Si está vacío, todas las
// peticiones se rechazan con 401.
func NewCallbackHandler(service callbackReceiver, token string, logger *logger.Logger) *CallbackHandler {
	return &CallbackHandler{
		service:      service,
		tokenDigest:  sha256.Sum256([]byte(token)),
		tokenEnabled: token != "",
		logger:       logger,
	}
}

func (h *CallbackHandler) ReceiveRecord(w http.ResponseWriter, r *http.Request) {
	// 1. Método.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCallbackError(w, http.StatusMethodNotAllowed, ingestionErrors.ErrCallbackMethodNotAllowed)
		return
	}

	// 2. Autenticación (el token nunca se registra).
	if reason, ok := h.authenticate(r); !ok {
		h.logger.Warn("[Callback] petición no autorizada", "reason", reason, "remote_addr", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", `Bearer realm="agatha-callback"`)
		writeCallbackError(w, http.StatusUnauthorized, ingestionErrors.ErrCallbackUnauthorized)
		return
	}

	// 3. Content-Type.
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeCallbackError(w, http.StatusUnsupportedMediaType, ingestionErrors.ErrCallbackUnsupportedMediaType)
		return
	}

	// 4. Tamaño del cuerpo (declarado y real).
	if r.ContentLength > MaxCallbackBodyBytes {
		writeCallbackError(w, http.StatusRequestEntityTooLarge, ingestionErrors.ErrCallbackPayloadTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxCallbackBodyBytes)

	// 5. Decodificación: un único objeto JSON.
	request, err := decodeCallback(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeCallbackError(w, http.StatusRequestEntityTooLarge, ingestionErrors.ErrCallbackPayloadTooLarge)
			return
		}

		h.logger.Warn("[Callback] cuerpo JSON inválido", "remote_addr", r.RemoteAddr, "error", err)

		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			sharedhttp.HandleError(w, appErr)
			return
		}
		sharedhttp.HandleError(w, ingestionErrors.NewInvalidCallbackPayloadError("JSON mal formado o con tipos inválidos"))
		return
	}

	// 6. Caso de uso, acotado al tiempo que espera Dataflow Labs.
	ctx, cancel := context.WithTimeout(r.Context(), callbackTimeout)
	defer cancel()

	response, err := h.service.Receive(ctx, request)
	if err != nil {
		sharedhttp.HandleError(w, err)
		return
	}

	sharedhttp.WriteJSON(w, http.StatusOK, response)
}

// authenticate compara el token Bearer en tiempo constante (sobre los digest,
// para no filtrar la longitud). Devuelve el motivo del rechazo, sin el token.
func (h *CallbackHandler) authenticate(r *http.Request) (string, bool) {
	if !h.tokenEnabled {
		return "AGATHA_CALLBACK_TOKEN no configurada", false
	}

	header := r.Header.Get("Authorization")
	if header == "" {
		return "falta la cabecera Authorization", false
	}

	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "esquema distinto de Bearer", false
	}

	token := strings.TrimSpace(header[len(bearerPrefix):])
	if token == "" {
		return "token vacío", false
	}

	digest := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(digest[:], h.tokenDigest[:]) != 1 {
		return "token incorrecto", false
	}

	return "", true
}

func decodeCallback(body io.Reader) (dtos.DataRecordCallback, error) {
	var request dtos.DataRecordCallback

	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return dtos.DataRecordCallback{}, err
	}

	// Rechaza contenido extra después del objeto.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("contenido adicional después del objeto JSON")
		}
		return dtos.DataRecordCallback{}, err
	}

	return request, nil
}

// writeCallbackError responde los errores HTTP propios del callback (401, 405,
// 413, 415), que shared/http no mapea, con el mismo formato {code, message}.
func writeCallbackError(w http.ResponseWriter, status int, appErr *appErrors.AppError) {
	sharedhttp.WriteJSON(w, status, map[string]any{
		"code":    appErr.Code,
		"message": appErr.Message,
	})
}
