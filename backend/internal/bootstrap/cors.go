package bootstrap

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	sharedHTTP "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// CORSAllowedOriginsEnv es la lista, separada por comas, de orígenes del
// navegador autorizados a consumir la API (p. ej. el frontend en Vite o Vercel).
const CORSAllowedOriginsEnv = "AGATHA_CORS_ALLOWED_ORIGINS"

// WithCORS envuelve el handler HTTP raíz con la política CORS de
// AGATHA_CORS_ALLOWED_ORIGINS. Debe llamarse después de NewPlatform, que carga
// el .env. Vacía deja la API sin CORS; un valor inválido (incluido "*") es error.
func WithCORS(handler http.Handler) (http.Handler, error) {
	log := logger.New("CORS", "Info")

	origins, err := sharedHTTP.ParseAllowedOrigins(os.Getenv(CORSAllowedOriginsEnv))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", CORSAllowedOriginsEnv, err)
	}

	if len(origins) == 0 {
		log.Warn("CORS deshabilitado: ningún origen de navegador podrá leer la API",
			"variable", CORSAllowedOriginsEnv)
	} else {
		log.Info("CORS habilitado", "allowed_origins", strings.Join(origins, ","))
	}

	return sharedHTTP.CORS(origins, handler), nil
}
