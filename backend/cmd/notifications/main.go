// Command notifications envía un push de prueba por FCM a UN celular, con el
// formato del contrato (docs/CONTRATO_PUSH_FCM.md §7), para comprobar la
// credencial y la entrega en la app sin esperar una alerta:
//
//	notifications -test-send -device CEL-1A2B3C [-level RED] [-kind NEW]
//	notifications -test-send -token <token FCM> [-level RED] [-kind NEW]
//
// -device busca el token que la app registró en push_devices (necesita
// DATABASE_URL) y respeta NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST. -token no usa la
// base: sirve antes de que la app pueda llegar al backend.
//
// Usa la configuración de FCM del .env: con FCM_VALIDATE_ONLY sin definir o en
// true, FCM valida el mensaje y el token pero NO lo entrega. Para que llegue
// al celular hay que correrlo con FCM_VALIDATE_ONLY=false.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	notificationPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// sendTimeout cubre el token OAuth2 y el envío (10 s cada solicitud).
const sendTimeout = 30 * time.Second

func main() {
	log := logger.New("NotificationsCLI", "Info")

	testSend := flag.Bool("test-send", false, "envía un push de prueba a un celular (obligatorio)")
	deviceID := flag.String("device", "", "deviceId registrado (CEL-XXXXXX); excluyente con -token")
	token := flag.String("token", "", "token FCM del celular; excluyente con -device")
	level := flag.String("level", "RED", "nivel: YELLOW, ORANGE o RED")
	kind := flag.String("kind", "NEW", "tipo: NEW o LEVEL_UP")
	flag.Parse()

	if !*testSend {
		log.Error("-test-send es obligatorio (hoy es la única operación del comando)")
		flag.Usage()
		os.Exit(2)
	}
	if (*deviceID == "") == (*token == "") {
		log.Error("hay que indicar -device o -token (uno de los dos)")
		os.Exit(2)
	}
	message, err := bootstrap.NewTestPushMessage(*level, *kind, time.Now())
	if err != nil {
		log.Error("parámetros inválidos", "error", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	target := *token
	if *deviceID != "" {
		platform, err := bootstrap.NewPlatform()
		if err != nil {
			os.Exit(1)
		}
		if target, err = bootstrap.TestPushToken(ctx, platform, *deviceID); err != nil {
			log.Error("no se puede enviar a ese celular", "error", err)
			os.Exit(1)
		}
	} else if err := godotenv.Load(); err != nil {
		log.Warn("no se encontró archivo .env; se utilizarán variables de entorno")
	}

	sender, err := bootstrap.NewTestPushSender(ctx)
	if err != nil {
		log.Error("no se pudo crear el cliente de FCM", "error", err)
		os.Exit(1)
	}

	err = sender.Send(ctx, target, message)
	switch {
	case errors.Is(err, notificationPorts.ErrPushTokenUnregistered):
		log.Error("FCM no reconoce el token (UNREGISTERED): la app debe registrar uno nuevo")
		os.Exit(1)
	case err != nil:
		log.Error("el push de prueba falló", "error", err)
		os.Exit(1)
	}

	if sender.ValidateOnly {
		fmt.Printf("FCM VALIDÓ el mensaje y el token (proyecto %s), pero NO lo entregó.\n", sender.ProjectID)
		fmt.Println("Para que llegue al celular: FCM_VALIDATE_ONLY=false.")
	} else {
		fmt.Printf("Push ENTREGADO a FCM (proyecto %s): %s %s, messageId %d.\n",
			sender.ProjectID, message.Level, message.Kind, message.MessageID)
	}
}
