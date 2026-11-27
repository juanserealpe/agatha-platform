package token

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestHS256IssueAndVerify(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	signer, err := NewHS256(testSecret, 30*24*time.Hour, fixedClock(now))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	token, expiresAt, err := signer.Issue("admin")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if want := now.Add(30 * 24 * time.Hour); !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, se esperaba %v", expiresAt, want)
	}
	if parts := strings.Split(token, "."); len(parts) != 3 {
		t.Fatalf("el token debe tener 3 partes, tiene %d", len(parts))
	}

	subject, err := signer.Verify(token)
	if err != nil || subject != "admin" {
		t.Fatalf("Verify = %q, %v; se esperaba admin", subject, err)
	}
}

func TestHS256Rejects(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	signer, _ := NewHS256(testSecret, time.Hour, fixedClock(now))
	token, _, _ := signer.Issue("admin")

	t.Run("vencido", func(t *testing.T) {
		later, _ := NewHS256(testSecret, time.Hour, fixedClock(now.Add(time.Hour)))
		if _, err := later.Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, se esperaba ErrInvalidToken", err)
		}
	})

	t.Run("firmado con otro secreto", func(t *testing.T) {
		other, _ := NewHS256([]byte("ffffffffffffffffffffffffffffffff"), time.Hour, fixedClock(now))
		if _, err := other.Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, se esperaba ErrInvalidToken", err)
		}
	})

	t.Run("payload alterado", func(t *testing.T) {
		parts := strings.Split(token, ".")
		payload, _ := json.Marshal(map[string]any{"sub": "otro", "iss": issuer, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
		forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + parts[2]
		if _, err := signer.Verify(forged); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, se esperaba ErrInvalidToken", err)
		}
	})

	t.Run("alg none", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
		parts := strings.Split(token, ".")
		if _, err := signer.Verify(header + "." + parts[1] + "."); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, se esperaba ErrInvalidToken", err)
		}
	})

	for _, malformed := range []string{"", "abc", "a.b", "a.b.c.d", "!!.!!.!!"} {
		t.Run("malformado "+malformed, func(t *testing.T) {
			if _, err := signer.Verify(malformed); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("error = %v, se esperaba ErrInvalidToken", err)
			}
		})
	}
}

func TestNewHS256Validation(t *testing.T) {
	if _, err := NewHS256([]byte("corto"), time.Hour, time.Now); !errors.Is(err, ErrWeakSecret) {
		t.Fatalf("error = %v, se esperaba ErrWeakSecret", err)
	}
	if _, err := NewHS256(testSecret, 0, time.Now); err == nil {
		t.Fatal("se esperaba error con TTL 0")
	}
}
