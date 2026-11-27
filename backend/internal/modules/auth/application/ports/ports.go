// Package ports define lo que el caso de uso de login necesita del exterior.
package ports

import "time"

// TokenIssuer firma un token de sesión para el sujeto y devuelve su vencimiento.
type TokenIssuer interface {
	Issue(subject string) (token string, expiresAt time.Time, err error)
}

// PasswordVerifier compara una contraseña contra su hash almacenado.
type PasswordVerifier interface {
	Matches(hash, password string) bool
}
