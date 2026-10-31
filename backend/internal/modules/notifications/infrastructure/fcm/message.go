// Package fcm es el Adapter del canal móvil: envía los PushMessage por la API
// FCM HTTP v1 con la cuenta de servicio del proyecto Firebase
// (docs/CONTRATO_PUSH_FCM.md). El JSON de la credencial nunca va al log ni a
// los errores.
package fcm

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// colombia es la zona de startedAt: el contrato la muestra con -05:00.
var colombia = time.FixedZone("COT", -5*3600)

// sendRequest es el cuerpo de messages:send.
type sendRequest struct {
	// ValidateOnly: FCM valida el mensaje y el token sin entregarlo (local).
	ValidateOnly bool    `json:"validate_only,omitempty"`
	Message      message `json:"message"`
}

// message es solo-datos (sin bloque notification): la app arma el texto en el
// idioma del celular y el mensaje llega aunque la app esté cerrada.
type message struct {
	Token   string            `json:"token"`
	Android androidConfig     `json:"android"`
	Data    map[string]string `json:"data"`
}

type androidConfig struct {
	Priority string `json:"priority"`
}

// newSendRequest arma el cuerpo según el contrato (§3): todos los valores son
// string y los opcionales vacíos se omiten.
func newSendRequest(token string, m entities.PushMessage, validateOnly bool) sendRequest {
	data := map[string]string{
		"alertId": m.AlertID,
		"level":   string(m.Level),
		"kind":    string(m.Kind),
	}
	optional := map[string]string{
		"sensorId":       m.SensorID,
		"pk":             m.PK,
		"event":          string(m.Event),
		"classification": string(m.Classification),
	}
	if !m.StartedAt.IsZero() {
		optional["startedAt"] = m.StartedAt.In(colombia).Format(time.RFC3339)
	}
	for k, v := range optional {
		if v != "" {
			data[k] = v
		}
	}
	return sendRequest{
		ValidateOnly: validateOnly,
		Message:      message{Token: token, Android: androidConfig{Priority: "HIGH"}, Data: data},
	}
}
