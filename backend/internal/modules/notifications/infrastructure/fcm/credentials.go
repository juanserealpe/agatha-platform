package fcm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// messagingScope es el permiso que necesita messages:send.
const messagingScope = "https://www.googleapis.com/auth/firebase.messaging"

// ErrNoCredential: no se configuró ninguna credencial. El push queda
// deshabilitado (NoopSender) y el canal web sigue funcionando.
var ErrNoCredential = errors.New("no hay credencial de FCM configurada")

// errInvalidCredential no envuelve el error del parser: podría citar parte del JSON.
var errInvalidCredential = errors.New("la credencial de FCM no es un JSON de cuenta de servicio válido")

// Config es la configuración del sender. Si hay archivo, tiene prioridad sobre
// el base64.
type Config struct {
	ProjectID                string
	ServiceAccountFile       string
	ServiceAccountJSONBase64 string
	// ValidateOnly hace que FCM valide sin entregar (para probar desde local).
	ValidateOnly bool
}

// HasCredential dice si se configuró alguna credencial.
func (c Config) HasCredential() bool {
	return strings.TrimSpace(c.ServiceAccountFile) != "" || strings.TrimSpace(c.ServiceAccountJSONBase64) != ""
}

// serviceAccountJSON lee la credencial: el archivo montado o, de respaldo, el base64.
func (c Config) serviceAccountJSON() ([]byte, error) {
	if path := strings.TrimSpace(c.ServiceAccountFile); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("leer la credencial de FCM (%s): %w", path, err)
		}
		return raw, nil
	}
	if encoded := strings.TrimSpace(c.ServiceAccountJSONBase64); encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, errors.New("la credencial de FCM en base64 no se pudo decodificar")
		}
		return raw, nil
	}
	return nil, ErrNoCredential
}

// tokenSource arma la fuente de tokens OAuth2 de la cuenta de servicio. No
// llama a Google: el primer token se pide en el primer envío.
func (c Config) tokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	raw, err := c.serviceAccountJSON()
	if err != nil {
		return nil, err
	}
	creds, err := google.CredentialsFromJSONWithType(ctx, raw, google.ServiceAccount, messagingScope)
	if err != nil {
		return nil, errInvalidCredential
	}
	return creds.TokenSource, nil
}
