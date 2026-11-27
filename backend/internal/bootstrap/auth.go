package bootstrap

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	alertHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/application/services"
	authHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/infrastructure/security"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/infrastructure/token"
	ingestionHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/http"
	notificationHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/realtime"
	sharedHTTP "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Variables de entorno de la autenticación.
const (
	// AuthModeEnv: off | log | enforce. Vacía = log (valida y registra, no bloquea).
	AuthModeEnv = "AGATHA_AUTH_MODE"
	// AdminUserEnv / AdminPasswordHashEnv / JWTSecretEnv: el ÚNICO usuario web.
	// Van las tres o ninguna. El hash se genera con go run ./cmd/auth -hash-password.
	AdminUserEnv         = "AGATHA_ADMIN_USER"
	AdminPasswordHashEnv = "AGATHA_ADMIN_PASSWORD_HASH"
	JWTSecretEnv         = "AGATHA_JWT_SECRET"
	// JWTTTLEnv: vigencia de la sesión web (duración de Go). Vacía = 720h.
	JWTTTLEnv = "AGATHA_JWT_TTL"
	// MobileAPIKeysEnv: keys de la app móvil separadas por comas (varias = rotación).
	MobileAPIKeysEnv = "AGATHA_MOBILE_API_KEYS"
	// AuthClientIPHeaderEnv: encabezado con la IP real (p. ej. X-Real-IP del
	// nginx) para el límite de intentos de login. Vacía = RemoteAddr.
	AuthClientIPHeaderEnv = "AGATHA_AUTH_CLIENT_IP_HEADER"
)

const (
	defaultJWTTTL    = 720 * time.Hour
	minJWTTTL        = time.Hour
	maxJWTTTL        = 2160 * time.Hour
	minMobileKeyLen  = 32
	loginMaxFailures = 10
	loginWindow      = 15 * time.Minute
)

// publicRoutes no exigen credencial: el callback valida su propio Bearer
// (AGATHA_CALLBACK_TOKEN) y el login es la puerta de entrada.
var publicRoutes = []string{
	ingestionHTTP.CallbackPath,
	authHTTP.RouteLogin,
}

// mobileRoutes es TODO lo que puede tocar la key móvil: lectura y gestión de
// alertas, registro del token push y lectura del dashboard. Nunca umbrales,
// políticas, exportación ni el stream web. La key vive en el APK y se puede
// extraer: este alcance es lo que limita el daño si se filtra.
var mobileRoutes = []string{
	alertHTTP.RouteActive,
	alertHTTP.RoutePending,
	alertHTTP.RouteSearch,
	alertHTTP.RouteDetail,
	alertHTTP.RouteDeviceHistory,
	alertHTTP.RouteClassify,
	alertHTTP.RouteLabel,
	alertHTTP.RouteObservation,
	notificationHTTP.RoutePushToken,
	RouteDashboardDevices,
	RouteDashboardMeasurements,
	RouteDashboardVariables,
	RouteDashboardSeries,
	authHTTP.RouteMe,
}

// queryTokenRoutes aceptan ?access_token= porque EventSource no manda encabezados.
var queryTokenRoutes = []string{realtime.RouteStream}

// AuthModule arma el login web y el middleware de autenticación.
type AuthModule struct {
	mode       sharedHTTP.AuthMode
	handler    *authHTTP.AuthHandler
	verifier   sharedHTTP.TokenVerifier
	mobileKeys []string
	log        *logger.Logger
}

// NewAuthModule lee la configuración del entorno (después de NewPlatform, que
// carga el .env). Un valor inválido es error de arranque: es preferible no
// levantar a levantar con una seguridad distinta de la configurada.
func NewAuthModule() (*AuthModule, error) {
	log := logger.New("Auth", "Info")

	mode, err := sharedHTTP.ParseAuthMode(os.Getenv(AuthModeEnv))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", AuthModeEnv, err)
	}

	mobileKeys, err := parseMobileKeys(os.Getenv(MobileAPIKeysEnv))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", MobileAPIKeysEnv, err)
	}

	login, verifier, err := newAdminLogin()
	if err != nil {
		return nil, err
	}
	if login == nil && mode == sharedHTTP.AuthModeEnforce {
		return nil, fmt.Errorf("%s=enforce requiere %s, %s y %s", AuthModeEnv, AdminUserEnv, AdminPasswordHashEnv, JWTSecretEnv)
	}

	limiter := authHTTP.NewLoginLimiter(loginMaxFailures, loginWindow, time.Now)
	var useCase authHTTP.LoginUseCase // nil (503) si el admin no está configurado
	if login != nil {
		useCase = login
	}
	handler := authHTTP.NewAuthHandler(useCase, limiter, log).
		WithClientIPHeader(os.Getenv(AuthClientIPHeaderEnv))

	switch {
	case mode == sharedHTTP.AuthModeOff:
		log.Warn("autenticación DESHABILITADA: la API queda abierta", "variable", AuthModeEnv)
	case mode == sharedHTTP.AuthModeLog:
		log.Warn("autenticación en modo log: se registran los rechazos pero NO se bloquean", "variable", AuthModeEnv)
	default:
		log.Info("autenticación obligatoria (enforce)")
	}
	if login == nil {
		log.Warn("login web sin configurar: POST /api/auth/login responde 503",
			"variables", strings.Join([]string{AdminUserEnv, AdminPasswordHashEnv, JWTSecretEnv}, ","))
	}
	if len(mobileKeys) == 0 && mode != sharedHTTP.AuthModeOff {
		log.Warn("sin keys móviles: la app no podrá autenticarse", "variable", MobileAPIKeysEnv)
	}

	return &AuthModule{mode: mode, handler: handler, verifier: verifier, mobileKeys: mobileKeys, log: log}, nil
}

// RegisterRoutes registra POST /api/auth/login y GET /api/auth/me.
func (m *AuthModule) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// Wrap envuelve el mux con el middleware. Debe quedar DENTRO de WithCORS.
func (m *AuthModule) Wrap(mux *http.ServeMux) http.Handler {
	return sharedHTTP.Auth(sharedHTTP.AuthPolicy{
		Mode:             m.mode,
		MobileKeys:       m.mobileKeys,
		Verifier:         m.verifier,
		RoutePattern:     func(r *http.Request) string { _, pattern := mux.Handler(r); return pattern },
		PublicRoutes:     publicRoutes,
		MobileRoutes:     mobileRoutes,
		QueryTokenRoutes: queryTokenRoutes,
		OnDenied: func(r *http.Request, pattern, reason string) {
			// Nunca se registra la credencial ni la query (puede traer access_token).
			m.log.Warn("petición sin credencial válida",
				"mode", string(m.mode), "method", r.Method, "path", r.URL.Path,
				"pattern", pattern, "reason", reason, "remote", r.RemoteAddr)
		},
	}, mux)
}

// newAdminLogin devuelve (nil, nil, nil) si no hay ningún dato del admin.
func newAdminLogin() (*services.LoginService, sharedHTTP.TokenVerifier, error) {
	user := strings.TrimSpace(os.Getenv(AdminUserEnv))
	encodedHash := strings.TrimSpace(os.Getenv(AdminPasswordHashEnv))
	secret := os.Getenv(JWTSecretEnv)

	if user == "" && encodedHash == "" && secret == "" {
		return nil, nil, nil
	}
	if user == "" || encodedHash == "" || secret == "" {
		return nil, nil, fmt.Errorf("%s, %s y %s van juntas: falta alguna",
			AdminUserEnv, AdminPasswordHashEnv, JWTSecretEnv)
	}

	hash, err := security.DecodePasswordHash(encodedHash)
	if err != nil {
		return nil, nil, err
	}

	ttl, err := parseJWTTTL(os.Getenv(JWTTTLEnv))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", JWTTTLEnv, err)
	}

	signer, err := token.NewHS256([]byte(secret), ttl, time.Now)
	if err != nil {
		return nil, nil, err
	}

	return services.NewLoginService(user, hash, signer, security.Bcrypt{}), signer, nil
}

func parseJWTTTL(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultJWTTTL, nil
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("duración inválida %q (ej.: 720h)", raw)
	}
	if ttl < minJWTTTL || ttl > maxJWTTTL {
		return 0, fmt.Errorf("%s fuera de rango [%s, %s]", ttl, minJWTTTL, maxJWTTTL)
	}
	return ttl, nil
}

var errShortMobileKey = errors.New("cada key móvil debe tener al menos 32 caracteres (generala con go run ./cmd/auth -generate-key)")

func parseMobileKeys(raw string) ([]string, error) {
	var keys []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if key == "" || seen[key] {
			continue
		}
		if len(key) < minMobileKeyLen {
			return nil, errShortMobileKey
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys, nil
}
