// Package fcm implementa ports.PushSender con Firebase Cloud Messaging (Admin
// SDK v4). Las credenciales son las Application Default Credentials: la
// variable GOOGLE_APPLICATION_CREDENTIALS apunta al JSON de la cuenta de
// servicio (el SDK toma de ahí también el project ID).
package fcm

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// CredentialsEnv es la variable estándar de Google con la ruta del JSON.
const CredentialsEnv = "GOOGLE_APPLICATION_CREDENTIALS"

// Sender envía los Push por FCM.
type Sender struct {
	client *messaging.Client
}

// NewSender crea el cliente de FCM con las Application Default Credentials.
func NewSender(ctx context.Context) (*Sender, error) {
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("inicializar Firebase (%s): %w", CredentialsEnv, err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("crear el cliente de FCM: %w", err)
	}
	return &Sender{client: client}, nil
}

// Send valida y envía el Push; devuelve el ID del mensaje en FCM.
func (s *Sender) Send(ctx context.Context, push entities.Push) (string, error) {
	if err := push.Validate(); err != nil {
		return "", err
	}
	id, err := s.client.Send(ctx, toMessage(push))
	if err != nil {
		return "", fmt.Errorf("enviar push por FCM: %w", err)
	}
	return id, nil
}

// toMessage arma el mensaje de FCM: notificación visible + datos, con
// prioridad alta en Android y iOS (son alertas operativas).
func toMessage(p entities.Push) *messaging.Message {
	return &messaging.Message{
		Topic:        p.Topic,
		Token:        p.Token,
		Notification: &messaging.Notification{Title: p.Title, Body: p.Body},
		Data:         p.Data,
		Android:      &messaging.AndroidConfig{Priority: "high"},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{"apns-priority": "10"},
			Payload: &messaging.APNSPayload{Aps: &messaging.Aps{Sound: "default"}},
		},
	}
}
