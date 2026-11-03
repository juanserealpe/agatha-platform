package fcm

import (
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

func TestToMessage_Topic(t *testing.T) {
	m := toMessage(entities.Push{
		Topic: "alerts",
		Title: "Alerta roja",
		Body:  "El índice 0.42 superó el límite 0.30",
		Data:  map[string]string{"alert_id": "7"},
	})

	if m.Topic != "alerts" || m.Token != "" {
		t.Fatalf("destino = topic %q / token %q", m.Topic, m.Token)
	}
	if m.Notification == nil || m.Notification.Title != "Alerta roja" || m.Notification.Body != "El índice 0.42 superó el límite 0.30" {
		t.Fatalf("notification = %+v", m.Notification)
	}
	if m.Data["alert_id"] != "7" {
		t.Fatalf("data = %v", m.Data)
	}
	if m.Android == nil || m.Android.Priority != "high" {
		t.Fatalf("android = %+v, se esperaba prioridad high", m.Android)
	}
	if m.APNS == nil || m.APNS.Headers["apns-priority"] != "10" {
		t.Fatalf("apns = %+v, se esperaba apns-priority 10", m.APNS)
	}
	if m.APNS.Payload == nil || m.APNS.Payload.Aps == nil || m.APNS.Payload.Aps.Sound != "default" {
		t.Fatalf("apns payload = %+v, se esperaba sonido default", m.APNS.Payload)
	}
}

func TestToMessage_Token(t *testing.T) {
	m := toMessage(entities.Push{Token: "device-token", Title: "t", Body: "b"})

	if m.Token != "device-token" || m.Topic != "" {
		t.Fatalf("destino = topic %q / token %q", m.Topic, m.Token)
	}
}
