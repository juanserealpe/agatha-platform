package entities

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

// ErrNotPushable: el aviso no tiene nivel o código válidos para el push. La
// política del canal debió descartarlo antes.
var ErrNotPushable = errors.New("el aviso no se puede enviar por push")

// PushMessage es el mensaje solo-datos que espera la app Android
// (docs/CONTRATO_PUSH_FCM.md §3). Los campos vacíos se omiten al enviarlo.
type PushMessage struct {
	// AlertID es el código de la alerta (ALR-000142): la app abre su detalle.
	AlertID  string
	Level    PushLevel
	Kind     PushKind
	SensorID string
	// PK es la abscisa sin el prefijo "PK" (34+470).
	PK             string
	Event          PushEvent
	StartedAt      time.Time
	Classification PushClassification
}

// NewPushMessage arma el mensaje de un aviso. deviceName es devices.name del
// punto (vacío si no se conoce): de ahí salen SensorID y PK.
func NewPushMessage(n AlertNotice, deviceName string) (PushMessage, error) {
	level, ok := PushLevelFor(n.Level)
	if !ok || n.AlertCode == "" {
		return PushMessage{}, ErrNotPushable
	}
	kind, ok := PushKindFor(n.Code)
	if !ok {
		return PushMessage{}, ErrNotPushable
	}
	m := PushMessage{
		AlertID:   n.AlertCode,
		Level:     level,
		Kind:      kind,
		SensorID:  deviceName,
		PK:        PKFromDeviceName(deviceName),
		StartedAt: n.StartedAt,
	}
	if event, ok := PushEventFor(n.EventType); ok {
		m.Event = event
	}
	if c, ok := PushClassificationFor(n.Classification); ok {
		m.Classification = c
	}
	return m, nil
}

// pkInName es la regla de extractPk del frontend ("PK34-470", "pk 34+470").
// RE2 no tiene lookahead: los metros no pueden ir seguidos de otro dígito.
var pkInName = regexp.MustCompile(`(?i)pk\s*(\d{1,3})\s*[+-]\s*(\d{3})(?:\D|$)`)

// PKFromDeviceName saca la abscisa del nombre del dispositivo ("Sensor
// PK34-470" → "34+470"). Vacío si el nombre no la trae.
//
// Mejora futura: la abscisa debería ser una columna propia del dispositivo.
func PKFromDeviceName(name string) string {
	match := pkInName.FindStringSubmatch(name)
	if match == nil {
		return ""
	}
	km, _ := strconv.Atoi(match[1])
	return strconv.Itoa(km) + "+" + match[2]
}
