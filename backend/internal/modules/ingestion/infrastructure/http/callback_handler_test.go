package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const testToken = "s3cr3t-callback-token"

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

const validBody = `{
  "event": "data_record",
  "record_id": "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f",
  "mp_id": 11556,
  "serial_id": "AGATHA02",
  "timestamp": "2026-10-08T16:15:41",
  "records": [
    {"device_id": 11556, "timestamp": "2026-10-08T16:15:41", "name": "env_temperature_c", "readings": 22.4},
    {"device_id": 11556, "timestamp": "2026-10-08T16:15:41", "name": "env_humidity_pct", "readings": 77.01},
    {"device_id": 11556, "timestamp": "2026-10-08T16:15:41", "name": "accel_drift_deg_cloud", "readings": 0.0}
  ]
}`

type fakeReceiver struct {
	calls    int
	received dtos.DataRecordCallback
	response dtos.CallbackResponse
	err      error
}

func (f *fakeReceiver) Receive(_ context.Context, request dtos.DataRecordCallback) (dtos.CallbackResponse, error) {
	f.calls++
	f.received = request
	return f.response, f.err
}

func newRequest(method, body string) *http.Request {
	r := httptest.NewRequest(method, CallbackPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+testToken)
	return r
}

func serve(t *testing.T, receiver *fakeReceiver, token string, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	handler := NewCallbackHandler(receiver, token, logger.New("test", "Info"))
	w := httptest.NewRecorder()
	handler.ReceiveRecord(w, r)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta no es JSON: %q", w.Body.String())
	}
	return body["code"]
}

func TestCallbackHandler_StoresValidRecord(t *testing.T) {
	receiver := &fakeReceiver{response: dtos.CallbackResponse{Status: dtos.CallbackStatusStored, RecordID: "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f", ReceivedRecords: 3}}

	w := serve(t, receiver, testToken, newRequest(http.MethodPost, validBody))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200: %s", w.Code, w.Body.String())
	}
	if receiver.calls != 1 {
		t.Fatalf("el servicio se llamó %d veces", receiver.calls)
	}
	// readings conserva el literal exacto (0.0 no se convierte en 0).
	got := receiver.received.Records
	if len(got) != 3 || got[0].Readings != "22.4" || got[1].Readings != "77.01" || got[2].Readings != "0.0" {
		t.Fatalf("readings mal decodificados: %+v", got)
	}

	var response dtos.CallbackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Status != dtos.CallbackStatusStored {
		t.Fatalf("respuesta inesperada: %s", w.Body.String())
	}
}

func TestCallbackHandler_DuplicateIsAcknowledged(t *testing.T) {
	receiver := &fakeReceiver{response: dtos.CallbackResponse{Status: dtos.CallbackStatusDuplicate}}

	w := serve(t, receiver, testToken, newRequest(http.MethodPost, validBody))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200 para un duplicado", w.Code)
	}
}

func TestCallbackHandler_AcceptsJSONWithCharset(t *testing.T) {
	r := newRequest(http.MethodPost, validBody)
	r.Header.Set("Content-Type", "application/json; charset=utf-8")

	if w := serve(t, &fakeReceiver{}, testToken, r); w.Code != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", w.Code)
	}
}

func TestCallbackHandler_AcceptsBearerSchemeCaseInsensitive(t *testing.T) {
	r := newRequest(http.MethodPost, validBody)
	r.Header.Set("Authorization", "bearer "+testToken)

	if w := serve(t, &fakeReceiver{}, testToken, r); w.Code != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200", w.Code)
	}
}

func TestCallbackHandler_RejectsOtherMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			receiver := &fakeReceiver{}
			w := serve(t, receiver, testToken, newRequest(method, validBody))

			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, se esperaba 405", w.Code)
			}
			if w.Header().Get("Allow") != http.MethodPost {
				t.Fatalf("Allow = %q, se esperaba POST", w.Header().Get("Allow"))
			}
			if receiver.calls != 0 {
				t.Fatal("no debe llamar al servicio")
			}
		})
	}
}

func TestCallbackHandler_Authentication(t *testing.T) {
	cases := map[string]struct {
		configured string
		header     string
	}{
		"sin cabecera":            {testToken, ""},
		"esquema Basic":           {testToken, "Basic " + testToken},
		"token vacío":             {testToken, "Bearer "},
		"token incorrecto":        {testToken, "Bearer otro-token"},
		"token con prefijo":       {testToken, "Bearer " + testToken + "x"},
		"variable no configurada": {"", "Bearer " + testToken},
		"ambos vacíos":            {"", "Bearer "},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			receiver := &fakeReceiver{}
			r := newRequest(http.MethodPost, validBody)
			r.Header.Del("Authorization")
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}

			w := serve(t, receiver, tc.configured, r)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, se esperaba 401", w.Code)
			}
			if w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("falta WWW-Authenticate")
			}
			if strings.Contains(w.Body.String(), testToken) {
				t.Fatal("la respuesta expone el token")
			}
			if receiver.calls != 0 {
				t.Fatal("no debe llamar al servicio")
			}
		})
	}
}

func TestCallbackHandler_RejectsNonJSONContentType(t *testing.T) {
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded", "application/jsonx"} {
		t.Run(contentType, func(t *testing.T) {
			r := newRequest(http.MethodPost, validBody)
			r.Header.Set("Content-Type", contentType)

			w := serve(t, &fakeReceiver{}, testToken, r)

			if w.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d, se esperaba 415", w.Code)
			}
		})
	}
}

func TestCallbackHandler_RejectsOversizedBody(t *testing.T) {
	padding := strings.Repeat(" ", MaxCallbackBodyBytes)
	receiver := &fakeReceiver{}

	w := serve(t, receiver, testToken, newRequest(http.MethodPost, validBody+padding))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, se esperaba 413", w.Code)
	}

	// Sin Content-Length (chunked) el límite se aplica al leer.
	r := newRequest(http.MethodPost, "")
	r.Body = io.NopCloser(strings.NewReader(`{"event": "` + strings.Repeat("a", MaxCallbackBodyBytes) + `"}`))
	r.ContentLength = -1
	if w := serve(t, receiver, testToken, r); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, se esperaba 413 en chunked", w.Code)
	}

	if receiver.calls != 0 {
		t.Fatal("no debe llamar al servicio")
	}
}

func TestCallbackHandler_RejectsMalformedBodies(t *testing.T) {
	cases := map[string]string{
		"vacío":               ``,
		"JSON roto":           `{"event": "data_record"`,
		"no es objeto":        `[1, 2, 3]`,
		"dos objetos":         validBody + `{}`,
		"mp_id como texto":    strings.Replace(validBody, `"mp_id": 11556`, `"mp_id": "11556"`, 1),
		"mp_id decimal":       strings.Replace(validBody, `"mp_id": 11556`, `"mp_id": 11556.5`, 1),
		"readings como texto": strings.Replace(validBody, `"readings": 22.4`, `"readings": "22.4"`, 1),
		"readings null":       strings.Replace(validBody, `"readings": 22.4`, `"readings": null`, 1),
		"readings booleano":   strings.Replace(validBody, `"readings": 22.4`, `"readings": true`, 1),
		"records no arreglo":  strings.Replace(validBody, `"records": [`, `"records": {"x": [`, 1),
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			receiver := &fakeReceiver{}

			w := serve(t, receiver, testToken, newRequest(http.MethodPost, body))

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, se esperaba 400: %s", w.Code, w.Body.String())
			}
			if receiver.calls != 0 {
				t.Fatal("no debe llamar al servicio")
			}
		})
	}
}

func TestCallbackHandler_MapsServiceErrors(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"payload inválido":   {ingestionErrors.NewInvalidCallbackPayloadError("event"), http.StatusBadRequest},
		"punto inexistente":  {appErrors.ErrDeviceNotFound, http.StatusNotFound},
		"falla consulta":     {ingestionErrors.ErrConsultingDevice, http.StatusInternalServerError},
		"falla persistencia": {errors.Join(ingestionErrors.ErrPersistingIngestionState, errors.New("db caída")), http.StatusInternalServerError},
		"error desconocido":  {errors.New("boom"), http.StatusInternalServerError},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := serve(t, &fakeReceiver{err: tc.err}, testToken, newRequest(http.MethodPost, validBody))

			if w.Code != tc.status {
				t.Fatalf("status = %d, se esperaba %d", w.Code, tc.status)
			}
			if strings.Contains(w.Body.String(), "db caída") {
				t.Fatal("la respuesta expone el error interno")
			}
		})
	}
}

func TestCallbackHandler_ErrorBodiesUseModuleCodes(t *testing.T) {
	if code := errorCode(t, serve(t, &fakeReceiver{}, "", newRequest(http.MethodPost, validBody))); code != string(ingestionErrors.CodeUnauthorized) {
		t.Fatalf("code = %q", code)
	}
	if code := errorCode(t, serve(t, &fakeReceiver{}, testToken, newRequest(http.MethodGet, ""))); code != string(ingestionErrors.CodeMethodNotAllowed) {
		t.Fatalf("code = %q", code)
	}
}
