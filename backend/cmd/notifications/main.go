// Command notifications envía un push de prueba REAL por FCM para comprobar
// que las credenciales y las apps móviles funcionan. No usa la BD.
//
//	notifications -test-send [-topic alerts | -token <fcm-token>] [-title …] [-body …]
//
// Sin -topic ni -token usa NOTIFICATIONS_PUSH_TOPIC (default "alerts").
// Requiere GOOGLE_APPLICATION_CREDENTIALS (ruta al JSON de la cuenta de
// servicio), en el entorno o en .env.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/fcm"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const sendTimeout = 30 * time.Second

func main() {
	log := logger.New("NotificationsCLI", "Info")

	testSend := flag.Bool("test-send", false, "envía un push de prueba real por FCM (obligatorio)")
	topic := flag.String("topic", "", "topic de FCM (default: NOTIFICATIONS_PUSH_TOPIC o \"alerts\")")
	token := flag.String("token", "", "token FCM de un dispositivo (excluyente con -topic)")
	title := flag.String("title", "AGATHA: prueba de notificación", "título del push")
	body := flag.String("body", "Si ves este mensaje, las notificaciones push funcionan.", "cuerpo del push")
	flag.Parse()

	if !*testSend {
		log.Error("-test-send es obligatorio (hoy es la única operación del comando)")
		flag.Usage()
		os.Exit(2)
	}
	if *topic != "" && *token != "" {
		log.Error("-topic y -token son excluyentes")
		os.Exit(2)
	}

	if err := godotenv.Load(); err != nil {
		log.Warn("no se encontró archivo .env; se utilizarán variables de entorno")
	}
	if os.Getenv(fcm.CredentialsEnv) == "" {
		log.Error(fcm.CredentialsEnv + " no está configurada (ruta al JSON de la cuenta de servicio)")
		os.Exit(1)
	}
	if *topic == "" && *token == "" {
		*topic = bootstrap.PushTopic()
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	sender, err := fcm.NewSender(ctx)
	if err != nil {
		log.Error("no se pudo crear el cliente de FCM", "error", err)
		os.Exit(1)
	}

	push := entities.Push{
		Topic: *topic,
		Token: *token,
		Title: *title,
		Body:  *body,
		Data:  map[string]string{"test": "true", "sent_at": time.Now().UTC().Format(time.RFC3339)},
	}
	id, err := sender.Send(ctx, push)
	if err != nil {
		log.Error("el push de prueba falló", "error", err)
		os.Exit(1)
	}

	target := "topic " + *topic
	if *token != "" {
		target = "token " + *token
	}
	fmt.Printf("Push enviado a %s\nID de FCM: %s\n", target, id)
}
