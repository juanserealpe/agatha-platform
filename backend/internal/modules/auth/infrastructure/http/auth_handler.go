// Package http expone el login del administrador web.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/application/services"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	sharedHTTP "github.com/juanserealpe/agatha-platform/backend/internal/shared/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const (
	RouteLogin = "POST /api/auth/login"
	RouteMe    = "GET /api/auth/me"

	maxLoginBody = 4 << 10
)

var (
	errInvalidBody = &appErrors.AppError{Code: appErrors.CodeInvalidInput, Message: "el cuerpo debe ser {\"username\": \"...\", \"password\": \"...\"}"}
	errTooMany     = &appErrors.AppError{Code: appErrors.CodeTooManyAttempts, Message: "demasiados intentos fallidos; espere antes de volver a intentar"}
	errNoPrincipal = &appErrors.AppError{Code: appErrors.CodeUnauthorized, Message: "se requiere autenticación"}
)

// LoginUseCase es lo que el handler necesita del caso de uso de login.
type LoginUseCase interface {
	Login(username, password string) (services.Session, error)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Username    string    `json:"username"`
}

type meResponse struct {
	Principal sharedHTTP.Principal `json:"principal"`
}

type AuthHandler struct {
	login          LoginUseCase
	limiter        *LoginLimiter
	log            *logger.Logger
	clientIPHeader string
}

// NewAuthHandler recibe el caso de uso; nil deja el login respondiendo 503.
func NewAuthHandler(login LoginUseCase, limiter *LoginLimiter, log *logger.Logger) *AuthHandler {
	return &AuthHandler{login: login, limiter: limiter, log: log}
}

// WithClientIPHeader toma la IP del cliente de ese encabezado (p. ej.
// X-Real-IP que pone el nginx) en lugar de RemoteAddr. Solo debe usarse si el
// backend no es alcanzable sin pasar por el proxy, porque el encabezado se
// falsifica si se llega directo.
func (h *AuthHandler) WithClientIPHeader(header string) *AuthHandler {
	h.clientIPHeader = strings.TrimSpace(header)
	return h
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(RouteLogin, h.Login)
	mux.HandleFunc(RouteMe, h.Me)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	if h.login == nil {
		sharedHTTP.HandleError(w, services.ErrNotConfigured)
		return
	}

	client := h.clientIP(r)
	if blocked, wait := h.limiter.Blocked(client); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		h.log.Warn("login bloqueado por intentos fallidos", "client", client)
		sharedHTTP.HandleError(w, errTooMany)
		return
	}

	request, err := decodeLogin(http.MaxBytesReader(w, r.Body, maxLoginBody))
	if err != nil {
		sharedHTTP.HandleError(w, errInvalidBody)
		return
	}

	session, err := h.login.Login(request.Username, request.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			h.limiter.Fail(client)
			h.log.Warn("login rechazado", "client", client)
		} else {
			h.log.Error("login falló", "error", err)
		}
		sharedHTTP.HandleError(w, err)
		return
	}

	h.limiter.Succeed(client)
	h.log.Info("login correcto", "client", client, "username", session.Username)
	sharedHTTP.WriteJSON(w, http.StatusOK, loginResponse{
		AccessToken: session.AccessToken,
		TokenType:   session.TokenType,
		ExpiresAt:   session.ExpiresAt.UTC(),
		Username:    session.Username,
	})
}

// Me devuelve quién es el llamador según la credencial (admin o mobile). Sirve
// al frontend para validar al arrancar si el token guardado sigue vigente.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	principal, ok := sharedHTTP.PrincipalFromContext(r.Context())
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		sharedHTTP.HandleError(w, errNoPrincipal)
		return
	}
	sharedHTTP.WriteJSON(w, http.StatusOK, meResponse{Principal: principal})
}

func (h *AuthHandler) clientIP(r *http.Request) string {
	if h.clientIPHeader != "" {
		if value := strings.TrimSpace(r.Header.Get(h.clientIPHeader)); value != "" {
			return value
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func decodeLogin(body io.Reader) (loginRequest, error) {
	var request loginRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return loginRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return loginRequest{}, errors.New("contenido adicional después del objeto JSON")
	}
	return request, nil
}
