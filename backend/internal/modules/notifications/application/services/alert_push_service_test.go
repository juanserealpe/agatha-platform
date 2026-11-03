package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeSender struct {
	sent []entities.Push
	err  error
}

func (f *fakeSender) Send(_ context.Context, p entities.Push) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, p)
	return "projects/agatha/messages/1", nil
}

func notice() entities.AlertNotice {
	return entities.AlertNotice{
		MessageID:        42,
		AlertID:          7,
		AlertCode:        "ALT-0007",
		MeasuringPointID: 11556,
		EventType:        "FUGA",
		EvaluatedAt:      time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC),
		Code:             "UP_ROJO",
		Level:            "ROJO",
		MaxLevel:         "ROJO",
		AlertState:       "ACTIVA",
		Title:            "Alerta roja",
		Body:             "El índice 0.42 superó el límite 0.30",
		Source:           "LIVE",
	}
}

func newService(sender *fakeSender) *AlertPushService {
	return NewAlertPushService(sender, "alerts", logger.New("NotificationsTest", "Error"))
}

func TestAlertPushService_SendsTheNoticeToTheTopic(t *testing.T) {
	sender := &fakeSender{}

	sent, err := newService(sender).Notify(context.Background(), notice())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !sent || len(sender.sent) != 1 {
		t.Fatalf("sent = %v, envíos = %d; se esperaba 1 envío", sent, len(sender.sent))
	}

	p := sender.sent[0]
	if p.Topic != "alerts" || p.Token != "" {
		t.Fatalf("destino = topic %q / token %q, se esperaba topic alerts", p.Topic, p.Token)
	}
	if p.Title != "Alerta roja" || p.Body != "El índice 0.42 superó el límite 0.30" {
		t.Fatalf("título/cuerpo = %q / %q", p.Title, p.Body)
	}

	want := map[string]string{
		"message_id":         "42",
		"alert_id":           "7",
		"alert_code":         "ALT-0007",
		"measuring_point_id": "11556",
		"event_type":         "FUGA",
		"evaluated_at":       "2026-11-03T14:00:00Z",
		"code":               "UP_ROJO",
		"level":              "ROJO",
		"max_level":          "ROJO",
		"alert_state":        "ACTIVA",
		"source":             "LIVE",
	}
	if len(p.Data) != len(want) {
		t.Fatalf("data = %v, se esperaba %v", p.Data, want)
	}
	for k, v := range want {
		if p.Data[k] != v {
			t.Fatalf("data[%q] = %q, se esperaba %q", k, p.Data[k], v)
		}
	}
}

func TestAlertPushService_SkipsLateNotices(t *testing.T) {
	sender := &fakeSender{}
	n := notice()
	n.Late = true

	sent, err := newService(sender).Notify(context.Background(), n)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if sent || len(sender.sent) != 0 {
		t.Fatalf("sent = %v, envíos = %d; un aviso atrasado no se envía", sent, len(sender.sent))
	}
}

func TestAlertPushService_PropagatesSenderErrors(t *testing.T) {
	boom := errors.New("fcm caído")

	sent, err := newService(&fakeSender{err: boom}).Notify(context.Background(), notice())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, se esperaba %v", err, boom)
	}
	if sent {
		t.Fatal("sent = true con error del sender")
	}
}
