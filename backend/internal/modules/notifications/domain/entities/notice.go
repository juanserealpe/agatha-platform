// Package entities es el modelo del módulo de notificaciones: su vista propia
// de los eventos de Alert Management y el mensaje que espera la app móvil.
package entities

import "time"

// AlertNotice es la vista de Notifications de un aviso de alerta
// (alert_management.alert_notification): una alerta nueva o una subida de
// nivel. Lo reciben el canal web (banner) y el push.
type AlertNotice struct {
	// MessageID es la clave de idempotencia (un aviso por mensaje).
	MessageID        int64
	AlertID          int64
	AlertCode        string
	MeasuringPointID int64
	EventType        string
	StartedAt        time.Time
	EvaluatedAt      time.Time
	// Code es el código del catálogo (OPEN, UP_ROJO, …).
	Code     string
	Level    string
	MaxLevel string
	// Classification: CONFIRMADA o FALSA_ALERTA; vacío = sin clasificar.
	Classification string
	Title          string
	Body           string
	// Late: la hora llegó con atraso (catch-up).
	Late       bool
	OccurredAt time.Time
}

// AlertChange es la vista de Notifications de un cambio visible de una alerta
// (alert_management.alert_changed). Solo la usa el canal web (mapa y panel).
type AlertChange struct {
	AlertID          int64
	AlertCode        string
	MeasuringPointID int64
	EventType        string
	// Change: OPENED, LEVEL_RAISED, CLASSIFIED, …
	Change           string
	State            string
	CurrentLevel     string
	MaxLevel         string
	Blinking         bool
	ManagementStatus string
	Classification   string
	StartedAt        time.Time
	EvaluatedAt      time.Time
	OccurredAt       time.Time
}
