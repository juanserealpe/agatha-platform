package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	notificationHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var registeredAt = time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

func newMux(tokens notificationHTTP.PushTokens) *http.ServeMux {
	mux := http.NewServeMux()
	notificationHTTP.NewPushTokenHandler(tokens, logger.New("NotificationsTest", "Error")).RegisterRoutes(mux)
	return mux
}

func newServiceMux(store *memory.PushDeviceStore) *http.ServeMux {
	return newMux(services.NewPushTokenService(store, ports.ClockFunc(func() time.Time { return registeredAt }),
		logger.New("NotificationsTest", "Error")))
}

func put(mux *http.ServeMux, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/devices/push-token", strings.NewReader(body)))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct{ Code, Message string }
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("cuerpo de error: %v", err)
	}
	if body.Message == "" {
		t.Fatal("el error no trae message")
	}
	return body.Code
}

// La app repite el mismo registro hasta recibir 2xx: cada intento es 204.
func TestPushTokenHandler_RegistersIdempotently(t *testing.T) {
	store := memory.NewPushDeviceStore()
	mux := newServiceMux(store)
	for range 2 {
		if rec := put(mux, `{"deviceId":"CEL-1A2B3C","token":"token-fcm","platform":"ANDROID"}`); rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, cuerpo %s", rec.Code, rec.Body)
		}
	}
	got, _ := store.List(context.Background())
	want := []entities.PushDevice{{DeviceID: "CEL-1A2B3C", Token: "token-fcm", Platform: entities.PushPlatformAndroid, UpdatedAt: registeredAt}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
}

func TestPushTokenHandler_ReassignsATokenToTheNewDevice(t *testing.T) {
	store := memory.NewPushDeviceStore()
	mux := newServiceMux(store)
	put(mux, `{"deviceId":"CEL-1","token":"compartido","platform":"ANDROID"}`)
	if rec := put(mux, `{"deviceId":"CEL-2","token":"compartido","platform":"ANDROID"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	got, _ := store.List(context.Background())
	if len(got) != 1 || got[0].DeviceID != "CEL-2" {
		t.Fatalf("celulares = %+v", got)
	}
}

// Un campo nuevo de la app no debe dejarla reintentando para siempre.
func TestPushTokenHandler_IgnoresUnknownFields(t *testing.T) {
	rec := put(newServiceMux(memory.NewPushDeviceStore()), `{"deviceId":"CEL-1","token":"t","platform":"android","appVersion":"1.4.0"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, cuerpo %s", rec.Code, rec.Body)
	}
}

func TestPushTokenHandler_RejectsInvalidBodies(t *testing.T) {
	cases := map[string]string{
		"JSON inválido":      `{"deviceId":`,
		"dos objetos":        `{"deviceId":"CEL-1","token":"t","platform":"ANDROID"}{}`,
		"sin token":          `{"deviceId":"CEL-1","platform":"ANDROID"}`,
		"plataforma iOS":     `{"deviceId":"CEL-1","token":"t","platform":"IOS"}`,
		"deviceId muy largo": `{"deviceId":"` + strings.Repeat("x", 129) + `","token":"t","platform":"ANDROID"}`,
		"cuerpo de más 4 KB": `{"deviceId":"CEL-1","token":"` + strings.Repeat("t", 5000) + `","platform":"ANDROID"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			store := memory.NewPushDeviceStore()
			rec := put(newServiceMux(store), body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, se esperaba 400", rec.Code)
			}
			if code := errorCode(t, rec); code != "INVALID_INPUT" {
				t.Fatalf("code = %s", code)
			}
			if got, _ := store.List(context.Background()); len(got) != 0 {
				t.Fatalf("se guardó %+v", got)
			}
		})
	}
}

type failingTokens struct{}

func (failingTokens) Register(context.Context, services.RegisterPushTokenCommand) error {
	return errors.New("base caída")
}

func TestPushTokenHandler_StorageErrorIs500(t *testing.T) {
	rec := put(newMux(failingTokens{}), `{"deviceId":"CEL-1","token":"t","platform":"ANDROID"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "INTERNAL_ERROR" {
		t.Fatalf("code = %s", code)
	}
}

func TestPushTokenHandler_OnlyAcceptsPut(t *testing.T) {
	mux := newServiceMux(memory.NewPushDeviceStore())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/devices/push-token", strings.NewReader(`{}`)))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}
