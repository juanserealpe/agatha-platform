package security

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndDecode(t *testing.T) {
	encoded, err := HashPassword("secreta", MinCost)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if strings.Contains(encoded, "$") {
		t.Fatalf("el valor para el env no debe contener $: %q", encoded)
	}

	hash, err := DecodePasswordHash(encoded)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !(Bcrypt{}).Matches(hash, "secreta") || (Bcrypt{}).Matches(hash, "otra") {
		t.Fatal("Matches no coincide con la contraseña original")
	}
}

func TestDecodePasswordHashRejects(t *testing.T) {
	weak, _ := bcrypt.GenerateFromPassword([]byte("x"), bcrypt.MinCost)

	cases := map[string]string{
		"no base64":              "$2a$12$abc",
		"base64 de algo no hash": base64.StdEncoding.EncodeToString([]byte("hola")),
		"costo bajo":             base64.StdEncoding.EncodeToString(weak),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodePasswordHash(value); !errors.Is(err, ErrInvalidPasswordHash) {
				t.Fatalf("error = %v, se esperaba ErrInvalidPasswordHash", err)
			}
		})
	}
}
