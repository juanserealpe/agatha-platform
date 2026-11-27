// Package services contiene el caso de uso de login del administrador web.
package services

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/application/ports"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

var (
	// ErrInvalidCredentials no distingue usuario de contraseña incorrectos.
	ErrInvalidCredentials = &appErrors.AppError{
		Code:    appErrors.CodeInvalidCredentials,
		Message: "usuario o contraseña incorrectos",
	}
	// ErrNotConfigured: faltan AGATHA_ADMIN_* o AGATHA_JWT_SECRET.
	ErrNotConfigured = &appErrors.AppError{
		Code:    appErrors.CodeAuthNotConfigured,
		Message: "el login web no está configurado en el servidor",
	}
)

// Session es la sesión emitida tras un login correcto.
type Session struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
	Username    string
}

// LoginService valida al ÚNICO usuario administrador configurado.
type LoginService struct {
	username     string
	passwordHash string
	issuer       ports.TokenIssuer
	passwords    ports.PasswordVerifier
}

func NewLoginService(username, passwordHash string, issuer ports.TokenIssuer, passwords ports.PasswordVerifier) *LoginService {
	return &LoginService{username: username, passwordHash: passwordHash, issuer: issuer, passwords: passwords}
}

// Login emite una sesión si usuario y contraseña coinciden. Compara la
// contraseña SIEMPRE, aunque el usuario no coincida, para que el tiempo de
// respuesta no revele si el usuario existe.
func (s *LoginService) Login(username, password string) (Session, error) {
	if s == nil {
		return Session{}, ErrNotConfigured
	}

	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.username)) == 1
	passwordOK := s.passwords.Matches(s.passwordHash, password)
	if !userOK || !passwordOK || username == "" {
		return Session{}, ErrInvalidCredentials
	}

	token, expiresAt, err := s.issuer.Issue(s.username)
	if err != nil {
		return Session{}, fmt.Errorf("firmar la sesión: %w", err)
	}

	return Session{AccessToken: token, TokenType: "Bearer", ExpiresAt: expiresAt, Username: s.username}, nil
}
