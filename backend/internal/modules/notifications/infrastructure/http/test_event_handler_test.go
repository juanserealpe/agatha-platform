package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	notificationHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeEmitter struct {
	got    []entities.TestEvent
	result ports.TestEventResult
	err    error
}

func (f *fakeEmitter) Emit(_ context.Context, e entities.TestEvent) (ports.TestEventResult, error) {
	f.got = append(f.got, e)
	return f.result, f.err
}

func postTestEvent(emitter ports.TestEventEmitter, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	notificationHTTP.NewTestEventHandler(emitter, logger.New("NotificationsTest", "Error")).RegisterRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/notifications/test-event", strings.NewReader(body)))
	return rec
}

func TestTestEventHandler_EmitsAndReturns202(t *testing.T) {
	emitter := &fakeEmitter{result: ports.TestEventResult{AlertID: -1, AlertCode: "ALR-PRUEBA-000001", MessageID: -2, Change: "OPENED"}}
	rec := postTestEvent(emitter, `{"measuring_point_id":11556,"level":"ROJO"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, cuerpo %s", rec.Code, rec.Body)
	}
	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["alert_id"] != float64(-1) || body["alert_code"] != "ALR-PRUEBA-000001" || body["message_id"] != float64(-2) || body["change"] != "OPENED" {
		t.Fatalf("respuesta = %v", body)
	}
	if len(emitter.got) != 1 || emitter.got[0].Code != "OPEN" || emitter.got[0].EventType != "FUGA" {
		t.Fatalf("emitido = %+v: debía llegar normalizado", emitter.got)
	}
}

func TestTestEventHandler_ChangeOnlyHasNullMessageID(t *testing.T) {
	emitter := &fakeEmitter{result: ports.TestEventResult{AlertID: -1, AlertCode: "ALR-PRUEBA-000001", Change: "FINISHED"}}
	rec := postTestEvent(emitter, `{"measuring_point_id":1,"alert_id":-1,"level":"ROJO","change":"FINISHED"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"message_id":null`) {
		t.Fatalf("status = %d, cuerpo %s", rec.Code, rec.Body)
	}
}

func TestTestEventHandler_RejectsInvalidRequests(t *testing.T) {
	cases := map[string]string{
		"JSON inválido":     `{`,
		"campo desconocido": `{"measuring_point_id":1,"level":"ROJO","nivel":"ROJO"}`,
		"evento inválido":   `{"measuring_point_id":1,"level":"VERDE"}`,
		"alerta real":       `{"measuring_point_id":1,"alert_id":142,"level":"ROJO"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			emitter := &fakeEmitter{}
			rec := postTestEvent(emitter, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", rec.Code)
			}
			if code := errorCode(t, rec); code != "INVALID_INPUT" {
				t.Fatalf("code = %s", code)
			}
			if len(emitter.got) != 0 {
				t.Fatal("no debía publicar")
			}
		})
	}
}

func TestTestEventHandler_EmitterErrorIs500(t *testing.T) {
	rec := postTestEvent(&fakeEmitter{err: errors.New("bus cerrado")}, `{"measuring_point_id":1,"level":"ROJO"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}
