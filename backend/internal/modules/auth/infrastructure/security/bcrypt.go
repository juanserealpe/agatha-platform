// Package security adapta bcrypt al puerto PasswordVerifier.
package security

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// MinCost es el costo bcrypt mínimo aceptado al arrancar.
const MinCost = 10

// ErrInvalidPasswordHash: AGATHA_ADMIN_PASSWORD_HASH no es un bcrypt en base64.
var ErrInvalidPasswordHash = errors.New("AGATHA_ADMIN_PASSWORD_HASH inválido")

// Bcrypt implementa ports.PasswordVerifier.
type Bcrypt struct{}

func (Bcrypt) Matches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashPassword genera el valor para AGATHA_ADMIN_PASSWORD_HASH: el hash bcrypt
// codificado en base64. Se codifica porque el hash lleva "$" ("$2a$12$..."),
// que godotenv y el env_file de Docker Compose pueden interpretar como
// variable y romper en silencio.
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(hash), nil
}

// DecodePasswordHash valida el valor de AGATHA_ADMIN_PASSWORD_HASH y devuelve
// el hash bcrypt listo para comparar.
func DecodePasswordHash(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", fmt.Errorf("%w: no es base64 (generalo con go run ./cmd/auth -hash-password)", ErrInvalidPasswordHash)
	}
	cost, err := bcrypt.Cost(raw)
	if err != nil {
		return "", fmt.Errorf("%w: no es un hash bcrypt", ErrInvalidPasswordHash)
	}
	if cost < MinCost {
		return "", fmt.Errorf("%w: costo %d menor que %d", ErrInvalidPasswordHash, cost, MinCost)
	}
	return string(raw), nil
}
