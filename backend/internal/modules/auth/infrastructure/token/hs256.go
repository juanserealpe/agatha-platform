// Package token implementa un JWT HS256 mínimo con la stdlib: un solo
// algoritmo fijo (nunca se lee "alg" para elegirlo), emisor fijo y vencimiento.
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	issuer         = "agatha-platform"
	minSecretBytes = 32
)

var (
	// ErrInvalidToken agrupa cualquier rechazo (formato, firma, emisor,
	// vencimiento): al cliente no se le dice cuál fue.
	ErrInvalidToken = errors.New("token inválido")
	// ErrWeakSecret: el secreto HMAC debe tener al menos 32 bytes.
	ErrWeakSecret = errors.New("AGATHA_JWT_SECRET debe tener al menos 32 bytes")
)

// encodedHeader es constante: solo se emite y se acepta HS256.
var encodedHeader = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

type claims struct {
	Subject   string `json:"sub"`
	Issuer    string `json:"iss"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// HS256 emite y valida tokens. Implementa ports.TokenIssuer y
// shared/http.TokenVerifier.
type HS256 struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewHS256(secret []byte, ttl time.Duration, now func() time.Time) (*HS256, error) {
	if len(secret) < minSecretBytes {
		return nil, ErrWeakSecret
	}
	if ttl <= 0 {
		return nil, errors.New("la vigencia del token debe ser positiva")
	}
	return &HS256{secret: secret, ttl: ttl, now: now}, nil
}

func (h *HS256) Issue(subject string) (string, time.Time, error) {
	now := h.now()
	expiresAt := now.Add(h.ttl).Truncate(time.Second)

	payload, err := json.Marshal(claims{
		Subject:   subject,
		Issuer:    issuer,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}

	signingInput := encodedHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signingInput + "." + h.sign(signingInput), expiresAt, nil
}

func (h *HS256) Verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != encodedHeader {
		return "", ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(h.sign(signingInput))) {
		return "", ErrInvalidToken
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidToken
	}
	var c claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", ErrInvalidToken
	}
	if c.Issuer != issuer || c.Subject == "" || h.now().Unix() >= c.ExpiresAt {
		return "", ErrInvalidToken
	}

	return c.Subject, nil
}

func (h *HS256) sign(input string) string {
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
