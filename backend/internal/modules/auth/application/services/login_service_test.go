package services

import (
	"errors"
	"testing"
	"time"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

type fakeIssuer struct {
	subject string
	fail    bool
}

func (f *fakeIssuer) Issue(subject string) (string, time.Time, error) {
	if f.fail {
		return "", time.Time{}, errors.New("falló la firma")
	}
	f.subject = subject
	return "signed-token", time.Date(2026, 12, 27, 0, 0, 0, 0, time.UTC), nil
}

type fakePasswords struct{ calls int }

func (f *fakePasswords) Matches(hash, password string) bool {
	f.calls++
	return hash == "HASH" && password == "secreta"
}

func TestLoginService(t *testing.T) {
	newService := func() (*LoginService, *fakeIssuer, *fakePasswords) {
		issuer, passwords := &fakeIssuer{}, &fakePasswords{}
		return NewLoginService("admin", "HASH", issuer, passwords), issuer, passwords
	}

	t.Run("credenciales correctas emiten sesión", func(t *testing.T) {
		service, issuer, _ := newService()
		session, err := service.Login("admin", "secreta")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if session.AccessToken != "signed-token" || session.TokenType != "Bearer" || session.Username != "admin" {
			t.Fatalf("sesión = %+v", session)
		}
		if issuer.subject != "admin" {
			t.Fatalf("subject = %q, se esperaba admin", issuer.subject)
		}
	})

	t.Run("contraseña incorrecta", func(t *testing.T) {
		service, _, _ := newService()
		_, err := service.Login("admin", "otra")
		assertCode(t, err, appErrors.CodeInvalidCredentials)
	})

	t.Run("usuario incorrecto igual compara la contraseña (sin atajo de tiempo)", func(t *testing.T) {
		service, _, passwords := newService()
		_, err := service.Login("root", "secreta")
		assertCode(t, err, appErrors.CodeInvalidCredentials)
		if passwords.calls != 1 {
			t.Fatalf("Matches llamado %d veces, se esperaba 1", passwords.calls)
		}
	})

	t.Run("campos vacíos", func(t *testing.T) {
		service, _, _ := newService()
		_, err := service.Login("", "")
		assertCode(t, err, appErrors.CodeInvalidCredentials)
	})

	t.Run("fallo al firmar no es credencial inválida", func(t *testing.T) {
		issuer := &fakeIssuer{fail: true}
		service := NewLoginService("admin", "HASH", issuer, &fakePasswords{})
		_, err := service.Login("admin", "secreta")
		var appErr *appErrors.AppError
		if err == nil || errors.As(err, &appErr) {
			t.Fatalf("error = %v, se esperaba un error interno sin AppError", err)
		}
	})

	t.Run("servicio sin configurar", func(t *testing.T) {
		var service *LoginService
		_, err := service.Login("admin", "secreta")
		assertCode(t, err, appErrors.CodeAuthNotConfigured)
	})
}

func assertCode(t *testing.T, err error, code appErrors.ErrorCode) {
	t.Helper()
	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, se esperaba código %s", err, code)
	}
}
