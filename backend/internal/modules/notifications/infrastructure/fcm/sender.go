package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const (
	sendURL = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	// requestTimeout acota cada envío (incluye pedir el token OAuth2).
	requestTimeout = 10 * time.Second
	// maxErrorBody: el cuerpo de error de FCM es corto; no se lee más.
	maxErrorBody = 64 << 10
	// unregistered es el errorCode de FCM para un token que ya no existe.
	unregistered = "UNREGISTERED"
)

// HTTPV1Sender implementa ports.PushSender con la API FCM HTTP v1.
type HTTPV1Sender struct {
	client       *http.Client
	endpoint     string
	validateOnly bool
}

// NewHTTPV1Sender arma el sender con la cuenta de servicio de cfg. No llama a
// Google: el token OAuth2 se pide (y se renueva) en los envíos.
func NewHTTPV1Sender(ctx context.Context, cfg Config) (*HTTPV1Sender, error) {
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		return nil, errors.New("falta el id del proyecto de FCM")
	}
	source, err := cfg.tokenSource(ctx)
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(ctx, source)
	client.Timeout = requestTimeout
	return newSender(client, fmt.Sprintf(sendURL, projectID), cfg.ValidateOnly), nil
}

func newSender(client *http.Client, endpoint string, validateOnly bool) *HTTPV1Sender {
	return &HTTPV1Sender{client: client, endpoint: endpoint, validateOnly: validateOnly}
}

// Send envía un mensaje a un token. Devuelve ports.ErrPushTokenUnregistered
// solo si FCM dice UNREGISTERED: un 404 sin ese código puede ser un id de
// proyecto mal configurado, y borrar los tokens por eso los perdería todos.
func (s *HTTPV1Sender) Send(ctx context.Context, token string, m entities.PushMessage) error {
	body, err := json.Marshal(newSendRequest(token, m, s.validateOnly))
	if err != nil {
		return fmt.Errorf("armar el mensaje de FCM: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("armar la solicitud a FCM: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return transportError(ctx, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBody))
		return nil
	}
	fcmErr := readError(resp.Body)
	switch {
	case fcmErr.errorCode() == unregistered:
		return ports.ErrPushTokenUnregistered
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return &ports.RetryablePushError{
			RetryAfter: retryAfter(resp.Header.Get("Retry-After"), time.Now()),
			Err:        fmt.Errorf("FCM respondió %d: %s", resp.StatusCode, fcmErr),
		}
	default:
		return fmt.Errorf("FCM rechazó el mensaje (%d): %s", resp.StatusCode, fcmErr)
	}
}

// transportError clasifica un fallo sin respuesta de FCM. Si Google rechazó la
// credencial (4xx al pedir el token), reintentar no sirve.
func transportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var retrieve *oauth2.RetrieveError
	if errors.As(err, &retrieve) && retrieve.Response != nil && retrieve.Response.StatusCode < 500 {
		return fmt.Errorf("Google rechazó la cuenta de servicio de FCM (%d %s)", retrieve.Response.StatusCode, oauthErrorCode(retrieve))
	}
	return &ports.RetryablePushError{Err: fmt.Errorf("no se pudo llegar a FCM: %w", err)}
}

// oauthErrorCode es el "error" de OAuth2 (invalid_grant, …). El flujo JWT de
// x/oauth2 no llena ErrorCode, así que se lee del cuerpo, que no trae secretos.
func oauthErrorCode(e *oauth2.RetrieveError) string {
	if e.ErrorCode != "" {
		return e.ErrorCode
	}
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(e.Body, &body)
	return body.Error
}

// fcmError es el cuerpo de error de la API v1 (google.rpc.Status).
type fcmError struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Type      string `json:"@type"`
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

func readError(body io.Reader) fcmError {
	var e fcmError
	_ = json.NewDecoder(io.LimitReader(body, maxErrorBody)).Decode(&e)
	return e
}

// errorCode es el código propio de FCM (UNREGISTERED, QUOTA_EXCEEDED, …).
func (e fcmError) errorCode() string {
	for _, d := range e.Error.Details {
		if strings.HasSuffix(d.Type, "google.firebase.fcm.v1.FcmError") && d.ErrorCode != "" {
			return d.ErrorCode
		}
	}
	return ""
}

func (e fcmError) String() string {
	parts := []string{}
	for _, p := range []string{e.Error.Status, e.errorCode(), e.Error.Message} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "sin detalle"
	}
	return strings.Join(parts, " / ")
}

// retryAfter interpreta Retry-After en segundos o como fecha HTTP. 0 si no viene
// o no se entiende.
func retryAfter(header string, now time.Time) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(header); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

// NoopSender es el sender sin credencial: no envía nada y lo deja en el log.
// El canal web no depende de él.
type NoopSender struct {
	log *logger.Logger
}

func NewNoopSender(log *logger.Logger) *NoopSender {
	return &NoopSender{log: log}
}

func (s *NoopSender) Send(_ context.Context, _ string, m entities.PushMessage) error {
	s.log.Info("[FCM] Push deshabilitado (sin credencial): mensaje no enviado", "alert", m.AlertID, "kind", m.Kind)
	return nil
}
