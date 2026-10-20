package bootstrap

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedHTTP "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
)

func TestWithCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Run("aplica los orígenes de la variable de entorno", func(t *testing.T) {
		t.Setenv(CORSAllowedOriginsEnv, "http://localhost:5173, http://127.0.0.1:5173")

		handler, err := WithCORS(next)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/dashboard/devices", nil)
		req.Header.Set("Origin", "http://127.0.0.1:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
			t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba http://127.0.0.1:5173", got)
		}
	})

	t.Run("variable vacía deja el handler sin CORS", func(t *testing.T) {
		t.Setenv(CORSAllowedOriginsEnv, "")

		handler, err := WithCORS(next)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/dashboard/devices", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba vacío", got)
		}
	})

	t.Run("comodín es un error de configuración", func(t *testing.T) {
		t.Setenv(CORSAllowedOriginsEnv, "*")

		if _, err := WithCORS(next); !errors.Is(err, sharedHTTP.ErrInvalidCORSOrigin) {
			t.Fatalf("error = %v, se esperaba ErrInvalidCORSOrigin", err)
		}
	})
}
