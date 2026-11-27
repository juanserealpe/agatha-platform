package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/infrastructure/security"
	sharedHTTP "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
)

const (
	testJWTSecret = "0123456789abcdef0123456789abcdef-secret"
	testMobileKey = "mobile-key-0123456789abcdef0123456789"
)

func setAuthEnv(t *testing.T, mode string, withAdmin bool) {
	t.Helper()
	t.Setenv(AuthModeEnv, mode)
	t.Setenv(MobileAPIKeysEnv, testMobileKey)
	t.Setenv(JWTTTLEnv, "")
	t.Setenv(AuthClientIPHeaderEnv, "")
	if !withAdmin {
		t.Setenv(AdminUserEnv, "")
		t.Setenv(AdminPasswordHashEnv, "")
		t.Setenv(JWTSecretEnv, "")
		return
	}
	hash, err := security.HashPassword("secreta", security.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	t.Setenv(AdminUserEnv, "admin")
	t.Setenv(AdminPasswordHashEnv, hash)
	t.Setenv(JWTSecretEnv, testJWTSecret)
}

// platformMux arma un mux con las mismas formas de patrón que las rutas reales.
func platformMux(t *testing.T, auth *AuthModule) http.Handler {
	t.Helper()
	ok := func(w http.ResponseWriter, r *http.Request) {
		principal, _ := sharedHTTP.PrincipalFromContext(r.Context())
		w.Header().Set("X-Principal", string(principal))
		w.WriteHeader(http.StatusOK)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ingestion/callback", ok)
	mux.HandleFunc("GET /api/alerts/{id}", ok)
	mux.HandleFunc("POST /api/alerts/{id}/classification", ok)
	mux.HandleFunc("POST /api/alerts/policies", ok)
	mux.HandleFunc("POST /api/rule-engine/thresholds/bulk", ok)
	mux.HandleFunc("PUT /api/devices/push-token", ok)
	mux.HandleFunc("GET /api/notifications/stream", ok)
	mux.HandleFunc(RouteDashboardDevices, ok)
	auth.RegisterRoutes(mux)
	return auth.Wrap(mux)
}

func do(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthModuleEnforceEndToEnd(t *testing.T) {
	setAuthEnv(t, "enforce", true)
	auth, err := NewAuthModule()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	h := platformMux(t, auth)

	login := do(h, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"secreta"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %s", login.Code, login.Body.String())
	}
	var session struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(login.Body.Bytes(), &session)
	bearer := map[string]string{"Authorization": "Bearer " + session.AccessToken}
	mobile := map[string]string{"X-API-Key": testMobileKey}

	cases := []struct {
		name, method, target string
		headers              map[string]string
		want                 int
	}{
		{"callback sigue público (tiene su token)", http.MethodPost, "/ingestion/callback", nil, http.StatusOK},
		{"sin credencial", http.MethodGet, "/api/alerts/ALR-1", nil, http.StatusUnauthorized},
		{"admin a umbrales", http.MethodPost, "/api/rule-engine/thresholds/bulk", bearer, http.StatusOK},
		{"admin al stream por query", http.MethodGet, "/api/notifications/stream?access_token=" + session.AccessToken, nil, http.StatusOK},
		{"admin /me", http.MethodGet, "/api/auth/me", bearer, http.StatusOK},
		{"móvil a detalle", http.MethodGet, "/api/alerts/ALR-1", mobile, http.StatusOK},
		{"móvil clasifica", http.MethodPost, "/api/alerts/ALR-1/classification", mobile, http.StatusOK},
		{"móvil registra push", http.MethodPut, "/api/devices/push-token", mobile, http.StatusOK},
		{"móvil lee dashboard", http.MethodGet, "/api/dashboard/devices", mobile, http.StatusOK},
		{"móvil /me", http.MethodGet, "/api/auth/me", mobile, http.StatusOK},
		{"móvil NO cambia umbrales", http.MethodPost, "/api/rule-engine/thresholds/bulk", mobile, http.StatusForbidden},
		{"móvil NO cambia políticas", http.MethodPost, "/api/alerts/policies", mobile, http.StatusForbidden},
		{"móvil NO abre el stream web", http.MethodGet, "/api/notifications/stream", mobile, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(h, tc.method, tc.target, "", tc.headers); rec.Code != tc.want {
				t.Fatalf("status = %d, se esperaba %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestAuthModuleLogModeDoesNotBlock(t *testing.T) {
	setAuthEnv(t, "", false) // vacío = log, sin admin configurado
	auth, err := NewAuthModule()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	h := platformMux(t, auth)

	if rec := do(h, http.MethodPost, "/api/alerts/policies", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("status = %d; en log no debe bloquear", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/auth/login", `{"username":"a","password":"b"}`, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("login sin configurar: status = %d, se esperaba 503", rec.Code)
	}
}

func TestNewAuthModuleConfigErrors(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"modo inválido", func(t *testing.T) { setAuthEnv(t, "strict", true) }},
		{"enforce sin admin", func(t *testing.T) { setAuthEnv(t, "enforce", false) }},
		{"admin a medias", func(t *testing.T) {
			setAuthEnv(t, "log", false)
			t.Setenv(AdminUserEnv, "admin")
		}},
		{"hash no base64", func(t *testing.T) {
			setAuthEnv(t, "log", true)
			t.Setenv(AdminPasswordHashEnv, "$2a$12$abc")
		}},
		{"secreto JWT corto", func(t *testing.T) {
			setAuthEnv(t, "log", true)
			t.Setenv(JWTSecretEnv, "corto")
		}},
		{"TTL inválido", func(t *testing.T) {
			setAuthEnv(t, "log", true)
			t.Setenv(JWTTTLEnv, "mucho")
		}},
		{"TTL fuera de rango", func(t *testing.T) {
			setAuthEnv(t, "log", true)
			t.Setenv(JWTTTLEnv, "8760h")
		}},
		{"key móvil corta", func(t *testing.T) {
			setAuthEnv(t, "log", true)
			t.Setenv(MobileAPIKeysEnv, "corta")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			if _, err := NewAuthModule(); err == nil {
				t.Fatal("se esperaba error de configuración")
			}
		})
	}
}

func TestParseMobileKeys(t *testing.T) {
	keys, err := parseMobileKeys(" " + testMobileKey + " ,, " + testMobileKey + "-2 ," + testMobileKey)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("keys = %v, se esperaban 2 sin vacíos ni repetidos", keys)
	}
}
