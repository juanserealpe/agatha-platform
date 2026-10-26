package contract

import "time"

// AlertNotification es un mensaje NOTIFICACION de la línea de tiempo de una
// alerta, ya renderizado con el catálogo (Title y Body listos para mostrar).
type AlertNotification struct {
	// MessageID es el ID en alert_messages (clave de idempotencia).
	MessageID        int64  `json:"message_id"`
	AlertID          int64  `json:"alert_id"`
	AlertCode        string `json:"alert_code"`
	MeasuringPointID int64  `json:"measuring_point_id"`
	EventType        string `json:"event_type"`
	// EvaluatedAt es la hora (UTC) que generó el mensaje.
	EvaluatedAt time.Time `json:"evaluated_at"`
	// Code es el código del catálogo (OPEN, UP_NARANJA_FAST, UP_ROJO, …).
	Code string `json:"code"`
	// Level es el nivel del mensaje; MaxLevel y AlertState, los de la alerta
	// después de procesar la hora.
	Level      string `json:"level"`
	MaxLevel   string `json:"max_level"`
	AlertState string `json:"alert_state"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	// IndicatorValue es el I de la hora (nil en horas sin cálculo).
	IndicatorValue *float64 `json:"indicator_value"`
	// Source es LIVE o CATCH_UP.
	Source string `json:"source"`
	// Late: la hora llegó con más de late_notification_max_hours de atraso
	// (política vigente). La especificación pide no enviar push en ese caso;
	// lo aplica el consumidor.
	Late bool `json:"late"`
	// PolicyVersionID es la versión de alert_policy_versions que decidió el mensaje.
	PolicyVersionID int64     `json:"policy_version_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	SchemaVersion   int       `json:"schema_version"`
}
