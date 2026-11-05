package fcm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var pushMessage = entities.PushMessage{AlertID: "ALR-000142", Level: entities.PushLevelRed, Kind: entities.PushKindNew}

// fcmServer simula messages:send con el status y el cuerpo dados.
func fcmServer(t *testing.T, status int, body string, header map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fcmErrorBody(status, errorCode string) string {
	return `{"error":{"code":0,"message":"detalle","status":"` + status + `","details":[` +
		`{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"` + errorCode + `"}]}}`
}

func send(srv *httptest.Server) error {
	return newSender(srv.Client(), srv.URL, false).Send(context.Background(), "token-fcm", pushMessage)
}

func TestSend_OK(t *testing.T) {
	var got sendRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("solicitud %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = io.WriteString(w, `{"name":"projects/p/messages/1"}`)
	}))
	defer srv.Close()
	if err := send(srv); err != nil {
		t.Fatal(err)
	}
	if got.Message.Token != "token-fcm" || got.Message.Data["alertId"] != "ALR-000142" {
		t.Fatalf("FCM recibió %+v", got)
	}
}

func TestSend_UnregisteredTokenIsReported(t *testing.T) {
	err := send(fcmServer(t, http.StatusNotFound, fcmErrorBody("NOT_FOUND", "UNREGISTERED"), nil))
	if !errors.Is(err, ports.ErrPushTokenUnregistered) {
		t.Fatalf("error = %v", err)
	}
}

// Un 404 sin UNREGISTERED (p. ej., el id del proyecto está mal) no borra tokens.
func TestSend_NotFoundWithoutUnregisteredKeepsTheToken(t *testing.T) {
	err := send(fcmServer(t, http.StatusNotFound, `{"error":{"code":404,"message":"Requested entity was not found.","status":"NOT_FOUND"}}`, nil))
	if err == nil || errors.Is(err, ports.ErrPushTokenUnregistered) {
		t.Fatalf("error = %v", err)
	}
	var retryable *ports.RetryablePushError
	if errors.As(err, &retryable) {
		t.Fatal("un 404 no se reintenta")
	}
}

func TestSend_RetryableResponses(t *testing.T) {
	cases := map[string]struct {
		status int
		header map[string]string
		want   time.Duration
	}{
		"429 con segundos":    {http.StatusTooManyRequests, map[string]string{"Retry-After": "7"}, 7 * time.Second},
		"503 sin Retry-After": {http.StatusServiceUnavailable, nil, 0},
		"500":                 {http.StatusInternalServerError, nil, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := send(fcmServer(t, c.status, fcmErrorBody("UNAVAILABLE", "UNAVAILABLE"), c.header))
			var retryable *ports.RetryablePushError
			if !errors.As(err, &retryable) {
				t.Fatalf("error = %v, se esperaba reintentable", err)
			}
			if retryable.RetryAfter != c.want {
				t.Fatalf("RetryAfter = %v, se esperaba %v", retryable.RetryAfter, c.want)
			}
		})
	}
}

func TestSend_BadRequestIsPermanent(t *testing.T) {
	err := send(fcmServer(t, http.StatusBadRequest, fcmErrorBody("INVALID_ARGUMENT", "INVALID_ARGUMENT"), nil))
	var retryable *ports.RetryablePushError
	if err == nil || errors.As(err, &retryable) || errors.Is(err, ports.ErrPushTokenUnregistered) {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "INVALID_ARGUMENT") {
		t.Fatalf("el error no trae el detalle de FCM: %v", err)
	}
}

func TestSend_NetworkErrorIsRetryable(t *testing.T) {
	srv := fcmServer(t, http.StatusOK, "", nil)
	srv.Close()
	var retryable *ports.RetryablePushError
	if err := send(srv); !errors.As(err, &retryable) {
		t.Fatalf("error = %v", err)
	}
}

func TestSend_CanceledContextIsNotRetryable(t *testing.T) {
	srv := fcmServer(t, http.StatusOK, "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := newSender(srv.Client(), srv.URL, false).Send(ctx, "t", pushMessage)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"":                              0,
		"30":                            30 * time.Second,
		"-5":                            0,
		"mañana":                        0,
		"Fri, 30 Oct 2026 14:01:00 GMT": time.Minute,
		"Fri, 30 Oct 2026 13:00:00 GMT": 0,
	}
	for header, want := range cases {
		if got := retryAfter(header, now); got != want {
			t.Errorf("retryAfter(%q) = %v, se esperaba %v", header, got, want)
		}
	}
}

// serviceAccount arma un JSON de cuenta de servicio con una llave nueva y el
// token_uri dado.
func serviceAccount(t *testing.T, tokenURI string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	raw, _ := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "agatha-test", "private_key_id": "k1",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email": "fcm@agatha-test.iam.gserviceaccount.com", "token_uri": tokenURI,
	})
	return raw
}

// La cuenta de servicio se cambia por un token OAuth2 y cada envío lo lleva
// como Bearer; el token se reutiliza entre envíos.
func TestHTTPV1Sender_AuthenticatesWithTheServiceAccount(t *testing.T) {
	var tokenRequests atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests.Add(1)
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || r.Form.Get("assertion") == "" {
			t.Errorf("solicitud de token: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"ya29.prueba","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenSrv.Close()
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ya29.prueba" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer fcmSrv.Close()

	path := filepath.Join(t.TempDir(), "cuenta.json")
	if err := os.WriteFile(path, serviceAccount(t, tokenSrv.URL), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := Config{ServiceAccountFile: path}.tokenSource(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sender := newSender(oauth2.NewClient(context.Background(), source), fcmSrv.URL, false)
	for range 2 {
		if err := sender.Send(context.Background(), "t", pushMessage); err != nil {
			t.Fatal(err)
		}
	}
	if n := tokenRequests.Load(); n != 1 {
		t.Fatalf("se pidieron %d tokens, se esperaba 1", n)
	}
}

// Si Google rechaza la credencial, reintentar no sirve.
func TestHTTPV1Sender_RejectedCredentialIsPermanent(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"Invalid JWT Signature."}`)
	}))
	defer tokenSrv.Close()
	path := filepath.Join(t.TempDir(), "cuenta.json")
	_ = os.WriteFile(path, serviceAccount(t, tokenSrv.URL), 0o600)
	source, err := Config{ServiceAccountFile: path}.tokenSource(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = newSender(oauth2.NewClient(context.Background(), source), "http://127.0.0.1:1", false).
		Send(context.Background(), "t", pushMessage)
	var retryable *ports.RetryablePushError
	if err == nil || errors.As(err, &retryable) || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewHTTPV1Sender_RequiresProjectAndCredential(t *testing.T) {
	if _, err := NewHTTPV1Sender(context.Background(), Config{ServiceAccountJSONBase64: "e30="}); err == nil {
		t.Fatal("sin proyecto debía fallar")
	}
	if _, err := NewHTTPV1Sender(context.Background(), Config{ProjectID: "agatha-5fe71"}); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("error = %v, se esperaba ErrNoCredential", err)
	}
}

func TestNoopSender_SendsNothing(t *testing.T) {
	if err := NewNoopSender(logger.New("NotificationsTest", "Error")).Send(context.Background(), "t", pushMessage); err != nil {
		t.Fatal(err)
	}
}
