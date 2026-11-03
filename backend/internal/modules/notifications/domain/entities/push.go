// Package entities son los tipos del módulo de notificaciones. No conoce a
// Alert Management ni a Firebase: bootstrap traduce el evento a AlertNotice y
// infrastructure/fcm traduce Push al mensaje de FCM.
package entities

import (
	"errors"
	"time"
)

// ErrInvalidPush indica un Push sin destino, con dos destinos o sin título.
var ErrInvalidPush = errors.New("notifications: push inválido")

// AlertNotice es un aviso de alerta ya renderizado (Title y Body listos para
// mostrar). Lo arma bootstrap desde alert_management.alert_notification.
type AlertNotice struct {
	// MessageID es el ID en alert_messages (clave de idempotencia para la app).
	MessageID        int64
	AlertID          int64
	AlertCode        string
	MeasuringPointID int64
	EventType        string
	// EvaluatedAt es la hora (UTC) que generó el aviso.
	EvaluatedAt time.Time
	Code        string
	Level       string
	MaxLevel    string
	AlertState  string
	Title       string
	Body        string
	// Source es LIVE o CATCH_UP.
	Source string
	// Late: la hora llegó con demasiado atraso; no se envía push.
	Late bool
}

// Push es un aviso para los dispositivos: a un topic de FCM o a un token
// (exactamente uno de los dos). Data viaja como payload de datos (solo strings).
type Push struct {
	Topic string
	Token string
	Title string
	Body  string
	Data  map[string]string
}

// Validate exige un único destino y un título.
func (p Push) Validate() error {
	switch {
	case p.Topic == "" && p.Token == "":
		return errors.Join(ErrInvalidPush, errors.New("falta el topic o el token"))
	case p.Topic != "" && p.Token != "":
		return errors.Join(ErrInvalidPush, errors.New("topic y token son excluyentes"))
	case p.Title == "":
		return errors.Join(ErrInvalidPush, errors.New("falta el título"))
	}
	return nil
}
