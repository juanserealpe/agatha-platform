package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testMobileKey  = "mobile-key-0123456789abcdef0123456789"
	testOtherKey   = "mobile-key-rotated-abcdef0123456789abcd"
	testAdminToken = "valid-admin-token"
)

type fakeVerifier struct{}

func (fakeVerifier) Verify(token string) (string, error) {
	if token == testAdminToken {
		return "admin", nil
	}
	return "", errors.New("token inválido")
}

type denial struct {
	pattern string
	reason  string
}

// newTestAuth arma el middleware sobre un mux real con las mismas formas de
// patrón que usa la plataforma.
func newTestAuth(t *testing.T, mode AuthMode, denials *[]denial) http.Handler {
	t.Helper()

	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, r *http.Request) {
		principal, _ := PrincipalFromContext(r.Context())
		w.Header().Set("X-Principal", string(principal))
		w.WriteHeader(http.StatusOK)
	}
	mux.HandleFunc("/ingestion/callback", ok)
	mux.HandleFunc("POST /api/auth/login", ok)
	mux.HandleFunc("GET /api/alerts/{id}", ok)
	mux.HandleFunc("POST /api/alerts/policies", ok)
	mux.HandleFunc("GET /api/notifications/stream", ok)

	policy := AuthPolicy{
		Mode:             mode,
		MobileKeys:       []string{testMobileKey, testOtherKey},
		Verifier:         fakeVerifier{},
		RoutePattern:     func(r *http.Request) string { _, p := mux.Handler(r); return p },
		PublicRoutes:     []string{"/ingestion/callback", "POST /api/auth/login"},
		MobileRoutes:     []string{"GET /api/alerts/{id}"},
		QueryTokenRoutes: []string{"GET /api/notifications/stream"},
		OnDenied: func(r *http.Request, pattern, reason string) {
			if denials != nil {
				*denials = append(*denials, denial{pattern: pattern, reason: reason})
			}
		},
	}

	return Auth(policy, mux)
}

func serveAuth(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestAuthEnforce(t *testing.T) {
	handler := newTestAuth(t, AuthModeEnforce, nil)

	cases := []struct {
		name          string
		method        string
		target        string
		headers       map[string]string
		wantStatus    int
		wantPrincipal string
	}{
		{"ruta pública sin credencial", http.MethodPost, "/ingestion/callback", nil, http.StatusOK, ""},
		{"login público", http.MethodPost, "/api/auth/login", nil, http.StatusOK, ""},
		{"sin credencial", http.MethodGet, "/api/alerts/ALR-1", nil, http.StatusUnauthorized, ""},
		{"key móvil válida en ruta móvil", http.MethodGet, "/api/alerts/ALR-1",
			map[string]string{"X-API-Key": testMobileKey}, http.StatusOK, "mobile"},
		{"key móvil rotada también vale", http.MethodGet, "/api/alerts/ALR-1",
			map[string]string{"X-API-Key": testOtherKey}, http.StatusOK, "mobile"},
		{"key móvil inválida", http.MethodGet, "/api/alerts/ALR-1",
			map[string]string{"X-API-Key": "otra"}, http.StatusUnauthorized, ""},
		{"key móvil fuera de su alcance", http.MethodPost, "/api/alerts/policies",
			map[string]string{"X-API-Key": testMobileKey}, http.StatusForbidden, ""},
		{"bearer admin válido", http.MethodPost, "/api/alerts/policies",
			map[string]string{"Authorization": "Bearer " + testAdminToken}, http.StatusOK, "admin"},
		{"bearer sin distinguir mayúsculas del esquema", http.MethodGet, "/api/alerts/ALR-1",
			map[string]string{"Authorization": "bearer " + testAdminToken}, http.StatusOK, "admin"},
		{"bearer inválido", http.MethodGet, "/api/alerts/ALR-1",
			map[string]string{"Authorization": "Bearer malo"}, http.StatusUnauthorized, ""},
		{"esquema distinto de bearer", http.MethodGet, "/api/alerts/ALR-1",
			map[string]string{"Authorization": "Basic abc"}, http.StatusUnauthorized, ""},
		{"token por query en el stream", http.MethodGet, "/api/notifications/stream?access_token=" + testAdminToken,
			nil, http.StatusOK, "admin"},
		{"token por query fuera del stream se ignora", http.MethodGet, "/api/alerts/ALR-1?access_token=" + testAdminToken,
			nil, http.StatusUnauthorized, ""},
		{"ruta inexistente exige credencial", http.MethodGet, "/api/nada", nil, http.StatusUnauthorized, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveAuth(handler, tc.method, tc.target, tc.headers)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, se esperaba %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get("X-Principal"); got != tc.wantPrincipal {
				t.Fatalf("principal = %q, se esperaba %q", got, tc.wantPrincipal)
			}
		})
	}
}

func TestAuthEnforceResponses(t *testing.T) {
	handler := newTestAuth(t, AuthModeEnforce, nil)

	t.Run("401 en JSON con WWW-Authenticate", func(t *testing.T) {
		rec := serveAuth(handler, http.MethodGet, "/api/alerts/ALR-1", nil)
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate = %q, se esperaba Bearer", got)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, se esperaba application/json", got)
		}
		if body := rec.Body.String(); !containsAll(body, `"code":"UNAUTHORIZED"`) {
			t.Fatalf("body = %s, se esperaba code UNAUTHORIZED", body)
		}
	})

	t.Run("403 en JSON", func(t *testing.T) {
		rec := serveAuth(handler, http.MethodPost, "/api/alerts/policies", map[string]string{"X-API-Key": testMobileKey})
		if body := rec.Body.String(); !containsAll(body, `"code":"FORBIDDEN"`) {
			t.Fatalf("body = %s, se esperaba code FORBIDDEN", body)
		}
	})
}

func TestAuthLogMode(t *testing.T) {
	var denials []denial
	handler := newTestAuth(t, AuthModeLog, &denials)

	t.Run("sin credencial deja pasar y registra", func(t *testing.T) {
		denials = nil
		rec := serveAuth(handler, http.MethodGet, "/api/alerts/ALR-1", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, se esperaba 200", rec.Code)
		}
		if rec.Header().Get("X-Principal") != "" {
			t.Fatalf("no debería asignar principal sin credencial")
		}
		if len(denials) != 1 || denials[0].pattern != "GET /api/alerts/{id}" {
			t.Fatalf("denials = %+v, se esperaba un rechazo registrado", denials)
		}
	})

	t.Run("fuera de alcance deja pasar y registra", func(t *testing.T) {
		denials = nil
		rec := serveAuth(handler, http.MethodPost, "/api/alerts/policies", map[string]string{"X-API-Key": testMobileKey})
		if rec.Code != http.StatusOK || len(denials) != 1 {
			t.Fatalf("status = %d, denials = %+v", rec.Code, denials)
		}
	})

	t.Run("credencial válida asigna principal y no registra", func(t *testing.T) {
		denials = nil
		rec := serveAuth(handler, http.MethodGet, "/api/alerts/ALR-1", map[string]string{"X-API-Key": testMobileKey})
		if rec.Header().Get("X-Principal") != "mobile" || len(denials) != 0 {
			t.Fatalf("principal = %q, denials = %+v", rec.Header().Get("X-Principal"), denials)
		}
	})
}

func TestAuthOffMode(t *testing.T) {
	var denials []denial
	handler := newTestAuth(t, AuthModeOff, &denials)

	rec := serveAuth(handler, http.MethodPost, "/api/alerts/policies", nil)
	if rec.Code != http.StatusOK || len(denials) != 0 {
		t.Fatalf("status = %d, denials = %+v; off no valida nada", rec.Code, denials)
	}
}

func TestParseAuthMode(t *testing.T) {
	cases := map[string]AuthMode{
		"":          AuthModeLog,
		"off":       AuthModeOff,
		" LOG ":     AuthModeLog,
		"Enforce":   AuthModeEnforce,
		"enforce\n": AuthModeEnforce,
	}
	for raw, want := range cases {
		got, err := ParseAuthMode(raw)
		if err != nil || got != want {
			t.Fatalf("ParseAuthMode(%q) = %q, %v; se esperaba %q", raw, got, err, want)
		}
	}

	if _, err := ParseAuthMode("strict"); !errors.Is(err, ErrInvalidAuthMode) {
		t.Fatalf("error = %v, se esperaba ErrInvalidAuthMode", err)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
