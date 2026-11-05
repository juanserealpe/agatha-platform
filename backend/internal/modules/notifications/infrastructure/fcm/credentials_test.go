package fcm

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_HasCredential(t *testing.T) {
	if (Config{}).HasCredential() || (Config{ServiceAccountFile: "  "}).HasCredential() {
		t.Fatal("sin credencial")
	}
	if !(Config{ServiceAccountJSONBase64: "e30="}).HasCredential() {
		t.Fatal("con base64")
	}
}

// El archivo montado tiene prioridad sobre el base64.
func TestConfig_FileTakesPriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cuenta.json")
	_ = os.WriteFile(path, []byte(`{"desde":"archivo"}`), 0o600)
	raw, err := Config{ServiceAccountFile: path,
		ServiceAccountJSONBase64: base64.StdEncoding.EncodeToString([]byte(`{"desde":"base64"}`))}.serviceAccountJSON()
	if err != nil || string(raw) != `{"desde":"archivo"}` {
		t.Fatalf("credencial = %s, %v", raw, err)
	}
}

func TestConfig_Base64Fallback(t *testing.T) {
	raw, err := Config{ServiceAccountJSONBase64: base64.StdEncoding.EncodeToString([]byte(`{"a":1}`))}.serviceAccountJSON()
	if err != nil || string(raw) != `{"a":1}` {
		t.Fatalf("credencial = %s, %v", raw, err)
	}
}

func TestConfig_Errors(t *testing.T) {
	if _, err := (Config{}).serviceAccountJSON(); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("sin credencial: %v", err)
	}
	if _, err := (Config{ServiceAccountFile: filepath.Join(t.TempDir(), "no-existe.json")}).serviceAccountJSON(); err == nil {
		t.Fatal("archivo inexistente")
	}
	if _, err := (Config{ServiceAccountJSONBase64: "%%%"}).serviceAccountJSON(); err == nil {
		t.Fatal("base64 inválido")
	}
}

// Un JSON roto o de otro tipo se rechaza sin citar su contenido en el error.
func TestConfig_InvalidCredentialDoesNotLeakItsContent(t *testing.T) {
	cases := map[string]string{
		"JSON roto":       `{"private_key":"SECRETO-123`,
		"no es de cuenta": `{"type":"authorized_user","client_secret":"SECRETO-123","refresh_token":"SECRETO-123"}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			encoded := base64.StdEncoding.EncodeToString([]byte(content))
			_, err := Config{ServiceAccountJSONBase64: encoded}.tokenSource(context.Background())
			if !errors.Is(err, errInvalidCredential) {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), "SECRETO") {
				t.Fatalf("el error filtra la credencial: %v", err)
			}
		})
	}
}
