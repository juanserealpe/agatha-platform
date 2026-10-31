package entities

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// PushPlatform es la plataforma del celular. Solo existe la app Android.
type PushPlatform string

const PushPlatformAndroid PushPlatform = "ANDROID"

// Límites del registro (el endpoint no tiene autenticación: riesgo 10 del plan).
const (
	MaxPushDeviceIDLength = 128
	MaxPushTokenLength    = 4096
)

// ErrInvalidPushDevice: el deviceId, el token o la plataforma no son válidos.
var ErrInvalidPushDevice = errors.New("dispositivo de push inválido")

// PushDevice es el token FCM vigente de un celular (docs/CONTRATO_PUSH_FCM.md
// §1). Un celular tiene un solo token, y un token es de un solo celular.
type PushDevice struct {
	// DeviceID es el id propio de la app (CEL-1A2B3C); la app no tiene login.
	DeviceID  string
	Token     string
	Platform  PushPlatform
	UpdatedAt time.Time
}

// NewPushDevice valida el registro que manda la app. La plataforma no
// distingue mayúsculas, como los enums del contrato.
func NewPushDevice(deviceID, token, platform string, at time.Time) (PushDevice, error) {
	deviceID = strings.TrimSpace(deviceID)
	token = strings.TrimSpace(token)
	switch {
	case deviceID == "":
		return PushDevice{}, fmt.Errorf("%w: falta deviceId", ErrInvalidPushDevice)
	case len(deviceID) > MaxPushDeviceIDLength:
		return PushDevice{}, fmt.Errorf("%w: deviceId supera %d caracteres", ErrInvalidPushDevice, MaxPushDeviceIDLength)
	case token == "":
		return PushDevice{}, fmt.Errorf("%w: falta token", ErrInvalidPushDevice)
	case len(token) > MaxPushTokenLength:
		return PushDevice{}, fmt.Errorf("%w: token supera %d caracteres", ErrInvalidPushDevice, MaxPushTokenLength)
	case !strings.EqualFold(strings.TrimSpace(platform), string(PushPlatformAndroid)):
		return PushDevice{}, fmt.Errorf("%w: plataforma no soportada", ErrInvalidPushDevice)
	}
	return PushDevice{DeviceID: deviceID, Token: token, Platform: PushPlatformAndroid, UpdatedAt: at.UTC()}, nil
}
