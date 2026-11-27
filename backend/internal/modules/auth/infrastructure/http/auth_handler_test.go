package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/application/services"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	sharedHTTP "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type stubLogin struct{ calls int }

func (s *stubLogin) Login(username, password string) (services.Session, error) {
	s.calls++
	if username == "admin" && password == "secreta" {
		return services.Session{
			AccessToken: "tok",
			TokenType:   "Bearer",
			ExpiresAt:   time.Date(2026, 12, 27, 0, 0, 0, 0, time.UTC),
			Username:    "admin",
		}, nil
	}
	return services.Session{}, services.ErrInvalidCredentials
}

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

func newHandler(login LoginUseCase, clock *testClock) *http.ServeMux {
	limiter := NewLoginLimiter(3, 15*time.Minute, clock.Now)
	handler := NewAuthHandler(login, limiter, logger.New("AuthTest", "Error"))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func postLogin(mux http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:5555"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestLogin(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}

	t.Run("200 con la sesión", func(t *testing.T) {
		rec := postLogin(newHandler(&stubLogin{}, clock), `{"username":"admin","password":"secreta"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		var got map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["accessToken"] != "tok" || got["tokenType"] != "Bearer" ||
			got["username"] != "admin" || got["expiresAt"] != "2026-12-27T00:00:00Z" {
			t.Fatalf("body = %v", got)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("Cache-Control = %q, se esperaba no-store", rec.Header().Get("Cache-Control"))
		}
	})

	t.Run("401 con credenciales incorrectas", func(t *testing.T) {
		rec := postLogin(newHandler(&stubLogin{}, clock), `{"username":"admin","password":"mala"}`)
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "INVALID_CREDENTIALS") {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
	})

	for name, body := range map[string]string{
		"JSON inválido":   `{"username":`,
		"campo extra":     `{"username":"admin","password":"x","role":"root"}`,
		"contenido extra": `{"username":"admin","password":"x"} {}`,
	} {
		t.Run("400 "+name, func(t *testing.T) {
			login := &stubLogin{}
			rec := postLogin(newHandler(login, clock), body)
			if rec.Code != http.StatusBadRequest || login.calls != 0 {
				t.Fatalf("status = %d, calls = %d", rec.Code, login.calls)
			}
		})
	}

	t.Run("429 tras superar los intentos fallidos y se libera al pasar la ventana", func(t *testing.T) {
		login := &stubLogin{}
		mux := newHandler(login, clock)
		for i := 0; i < 3; i++ {
			postLogin(mux, `{"username":"admin","password":"mala"}`)
		}

		rec := postLogin(mux, `{"username":"admin","password":"secreta"}`)
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("status = %d, Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
		}
		if login.calls != 3 {
			t.Fatalf("bloqueado no debe llegar al caso de uso: calls = %d", login.calls)
		}

		clock.now = clock.now.Add(16 * time.Minute)
		if rec := postLogin(mux, `{"username":"admin","password":"secreta"}`); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, se esperaba 200 pasada la ventana", rec.Code)
		}
	})

	t.Run("un login correcto limpia los fallos", func(t *testing.T) {
		mux := newHandler(&stubLogin{}, clock)
		postLogin(mux, `{"username":"admin","password":"mala"}`)
		postLogin(mux, `{"username":"admin","password":"mala"}`)
		postLogin(mux, `{"username":"admin","password":"secreta"}`)
		postLogin(mux, `{"username":"admin","password":"mala"}`)
		if rec := postLogin(mux, `{"username":"admin","password":"secreta"}`); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, se esperaba 200", rec.Code)
		}
	})

	t.Run("503 sin login configurado", func(t *testing.T) {
		rec := postLogin(newHandler(nil, clock), `{"username":"admin","password":"secreta"}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, se esperaba 503", rec.Code)
		}
	})
}

func TestMe(t *testing.T) {
	clock := &testClock{now: time.Now()}
	mux := newHandler(&stubLogin{}, clock)

	t.Run("devuelve el principal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req = req.WithContext(sharedHTTP.WithPrincipal(req.Context(), sharedHTTP.PrincipalMobile))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"principal":"mobile"`) {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("401 sin principal (modo log)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), string(appErrors.CodeUnauthorized)) {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
	})
}
