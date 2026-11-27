package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

// ErrInvalidAuthMode indica un valor de AGATHA_AUTH_MODE no admitido.
var ErrInvalidAuthMode = errors.New("modo de autenticación inválido")

// AuthMode decide qué hace el middleware con una petición sin credencial válida.
type AuthMode string

const (
	// AuthModeOff no valida nada (el handler queda igual que antes).
	AuthModeOff AuthMode = "off"
	// AuthModeLog valida y asigna principal, pero deja pasar lo rechazado y
	// solo lo registra: sirve para desplegar sin romper clientes viejos.
	AuthModeLog AuthMode = "log"
	// AuthModeEnforce responde 401/403 a lo rechazado.
	AuthModeEnforce AuthMode = "enforce"
)

// ParseAuthMode interpreta el modo sin distinguir mayúsculas ni espacios.
// Vacío equivale a log.
func ParseAuthMode(raw string) (AuthMode, error) {
	switch mode := AuthMode(strings.ToLower(strings.TrimSpace(raw))); mode {
	case "":
		return AuthModeLog, nil
	case AuthModeOff, AuthModeLog, AuthModeEnforce:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: %q (valores: off, log, enforce)", ErrInvalidAuthMode, raw)
	}
}

// Principal identifica QUIÉN hace la petición, según la credencial que trajo.
type Principal string

const (
	PrincipalAdmin  Principal = "admin"
	PrincipalMobile Principal = "mobile"
)

type principalKey struct{}

// PrincipalFromContext devuelve el principal que asignó el middleware. En modo
// log una petición sin credencial válida llega sin principal.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// WithPrincipal devuelve un contexto con el principal (útil en tests de handlers).
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// TokenVerifier valida un token de sesión web y devuelve su sujeto.
type TokenVerifier interface {
	Verify(token string) (string, error)
}

// Encabezado y parámetro de las credenciales.
const (
	APIKeyHeader     = "X-API-Key"
	AccessTokenQuery = "access_token"
	bearerScheme     = "bearer "
)

// AuthPolicy configura el middleware. Las rutas se expresan con el patrón
// EXACTO con el que se registraron en el ServeMux (p. ej. "GET /api/alerts/{id}").
type AuthPolicy struct {
	Mode AuthMode
	// MobileKeys: keys válidas de la app móvil. Más de una permite rotar.
	MobileKeys []string
	// Verifier valida el Bearer de la web. Nil: ningún Bearer es válido.
	Verifier TokenVerifier
	// RoutePattern resuelve el patrón del mux que atenderá la petición.
	RoutePattern func(*http.Request) string
	// PublicRoutes no exigen credencial (callback con su propio token, login).
	PublicRoutes []string
	// MobileRoutes: únicas rutas a las que llega la key móvil.
	MobileRoutes []string
	// QueryTokenRoutes aceptan el Bearer en ?access_token= (EventSource no
	// puede mandar encabezados). En el resto de rutas el parámetro se ignora.
	QueryTokenRoutes []string
	// OnDenied se llama por cada rechazo (en log y en enforce). Nunca recibe
	// la credencial, solo el motivo.
	OnDenied func(r *http.Request, pattern, reason string)
}

var (
	errUnauthorized = &appErrors.AppError{Code: appErrors.CodeUnauthorized, Message: "se requiere autenticación"}
	errForbidden    = &appErrors.AppError{Code: appErrors.CodeForbidden, Message: "la credencial no tiene acceso a esta ruta"}
)

// Auth envuelve next con la política de autenticación. NO deduce el tipo de
// cliente por encabezados como User-Agent u Origin (se falsifican): la
// credencial misma lo identifica. X-API-Key => móvil; Bearer => admin web.
//
// Debe quedar DENTRO de CORS: los preflight OPTIONS nunca traen credenciales y
// CORS los responde antes de llegar aquí.
func Auth(policy AuthPolicy, next http.Handler) http.Handler {
	if policy.Mode == AuthModeOff {
		return next
	}

	public := toSet(policy.PublicRoutes)
	mobileRoutes := toSet(policy.MobileRoutes)
	queryRoutes := toSet(policy.QueryTokenRoutes)
	keyDigests := make([][sha256.Size]byte, 0, len(policy.MobileKeys))
	for _, key := range policy.MobileKeys {
		keyDigests = append(keyDigests, sha256.Sum256([]byte(key)))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pattern := policy.RoutePattern(r)
		if public[pattern] {
			next.ServeHTTP(w, r)
			return
		}

		principal, status, reason := authenticate(r, pattern, policy.Verifier, keyDigests, mobileRoutes, queryRoutes)
		if status == 0 {
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
			return
		}

		if policy.OnDenied != nil {
			policy.OnDenied(r, pattern, reason)
		}

		if policy.Mode == AuthModeLog {
			next.ServeHTTP(w, r)
			return
		}

		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", "Bearer")
			HandleError(w, errUnauthorized)
			return
		}
		HandleError(w, errForbidden)
	})
}

// authenticate devuelve el principal, o el status de rechazo (401/403) y su motivo.
func authenticate(
	r *http.Request,
	pattern string,
	verifier TokenVerifier,
	keyDigests [][sha256.Size]byte,
	mobileRoutes, queryRoutes map[string]bool,
) (Principal, int, string) {
	if key := r.Header.Get(APIKeyHeader); key != "" {
		if !mobileKeyValid(key, keyDigests) {
			return "", http.StatusUnauthorized, "key móvil incorrecta"
		}
		if !mobileRoutes[pattern] {
			return "", http.StatusForbidden, "ruta fuera del alcance de la key móvil"
		}
		return PrincipalMobile, 0, ""
	}

	token, reason := bearerToken(r, queryRoutes[pattern])
	if token == "" {
		return "", http.StatusUnauthorized, reason
	}
	if verifier == nil {
		return "", http.StatusUnauthorized, "sesión web no configurada"
	}
	if _, err := verifier.Verify(token); err != nil {
		return "", http.StatusUnauthorized, "token web inválido o vencido"
	}
	return PrincipalAdmin, 0, ""
}

// mobileKeyValid compara en tiempo constante sobre los digest (no filtra la
// longitud) y recorre TODAS las keys para no filtrar cuál coincidió.
func mobileKeyValid(key string, digests [][sha256.Size]byte) bool {
	digest := sha256.Sum256([]byte(key))
	valid := 0
	for _, candidate := range digests {
		valid |= subtle.ConstantTimeCompare(digest[:], candidate[:])
	}
	return valid == 1
}

func bearerToken(r *http.Request, allowQuery bool) (string, string) {
	header := r.Header.Get("Authorization")
	if header == "" {
		if allowQuery {
			if token := r.URL.Query().Get(AccessTokenQuery); token != "" {
				return token, ""
			}
		}
		return "", "sin credencial"
	}

	if len(header) < len(bearerScheme) || !strings.EqualFold(header[:len(bearerScheme)], bearerScheme) {
		return "", "esquema distinto de Bearer"
	}
	token := strings.TrimSpace(header[len(bearerScheme):])
	if token == "" {
		return "", "token vacío"
	}
	return token, ""
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}
