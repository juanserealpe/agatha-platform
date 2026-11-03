package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ErrInvalidCORSOrigin indica un origen CORS mal formado o no admitido.
var ErrInvalidCORSOrigin = errors.New("origen CORS inválido")

// El navegador consume los GET de lectura y los POST de Alert Management y
// Rule Engine, sin autenticación: no se permiten otros métodos ni credenciales.
const (
	corsAllowedMethods = "GET, POST"
	corsAllowedHeaders = "Accept, Content-Type"
	corsMaxAgeSeconds  = "600"
)

var (
	corsAllowedMethodSet = map[string]bool{http.MethodGet: true, http.MethodPost: true}
	corsAllowedHeaderSet = map[string]bool{"accept": true, "content-type": true}
)

// ParseAllowedOrigins interpreta una lista de orígenes separada por comas
// (ignorando espacios, vacíos, repetidos y la barra final) y la normaliza a
// esquema://host[:puerto] en minúsculas. Rechaza "*" y cualquier valor con
// ruta, query o esquema distinto de http/https. Una lista vacía devuelve nil.
func ParseAllowedOrigins(raw string) ([]string, error) {
	var origins []string
	seen := map[string]bool{}

	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSuffix(strings.TrimSpace(part), "/")
		if value == "" {
			continue
		}
		if value == "*" {
			return nil, fmt.Errorf("%w: no se admite el comodín %q; liste los orígenes explícitamente", ErrInvalidCORSOrigin, value)
		}

		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("%w: %q (formato esperado: https://dominio[:puerto])", ErrInvalidCORSOrigin, value)
		}

		origin := strings.ToLower(u.Scheme + "://" + u.Host)
		if !seen[origin] {
			seen[origin] = true
			origins = append(origins, origin)
		}
	}

	return origins, nil
}

// CORS envuelve next y agrega los encabezados CORS solo para los orígenes de
// allowedOrigins. Los preflight (OPTIONS con Access-Control-Request-Method) se
// responden aquí: 204 si origen, método y encabezados están permitidos y 403 si
// no. Sin orígenes configurados devuelve next sin cambios.
func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}

	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// La respuesta depende del Origin: evita que un caché la sirva a otro origen.
		w.Header().Add("Vary", "Origin")
		originAllowed := allowed[origin]

		requestedMethod := r.Header.Get("Access-Control-Request-Method")
		if r.Method == http.MethodOptions && requestedMethod != "" {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")

			if !originAllowed || !corsAllowedMethodSet[requestedMethod] ||
				!headersAllowed(r.Header.Get("Access-Control-Request-Headers")) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Max-Age", corsMaxAgeSeconds)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if originAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		next.ServeHTTP(w, r)
	})
}

func headersAllowed(requested string) bool {
	for _, header := range strings.Split(requested, ",") {
		name := strings.ToLower(strings.TrimSpace(header))
		if name != "" && !corsAllowedHeaderSet[name] {
			return false
		}
	}
	return true
}
