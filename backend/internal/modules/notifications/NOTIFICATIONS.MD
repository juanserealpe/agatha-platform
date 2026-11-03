# Módulo: Notifications

**Responsabilidad**: entregar como **push (Firebase Cloud Messaging)** cada aviso que publica Alert Management (`alert_management.alert_notification` v1). Alert Management decide QUÉ es una notificación y su texto; este módulo decide CÓMO se entrega. No usa la BD, no guarda tokens ni reintenta.

| Trigger | Acción | Componente |
|---|---|---|
| Evento `alert_management.alert_notification` (suscripción `notifications.alert_notification`) | `late = true` ⇒ no se envía (regla de la especificación). Si no, un push al topic `NOTIFICATIONS_PUSH_TOPIC` con `title`/`body` del catálogo + datos. Un error se registra y no se reintenta (best-effort; la fuente de verdad es `alert_messages` `NOTIFICACION`) | `AlertNotificationSubscriber` → `AlertPushService.Notify` → `fcm.Sender` |
| `cmd/notifications -test-send [-topic …\|-token …] [-title …] [-body …]` | Push de prueba REAL (sin BD) para comprobar credenciales y apps | `fcm.Sender` |

## Push (FCM)

- **Destino**: topic `NOTIFICATIONS_PUSH_TOPIC` (default `alerts`). La app móvil se suscribe con `FirebaseMessaging.subscribeToTopic("alerts")`. No hay control por usuario.
- **Notificación**: `title` y `body` tal como vienen del catálogo de Alert Management.
- **Datos** (todos string): `message_id` (idempotencia: la app descarta repetidos), `alert_id`, `alert_code`, `measuring_point_id`, `event_type`, `evaluated_at` (RFC3339 UTC), `code`, `level`, `max_level`, `alert_state`, `source` (`LIVE`/`CATCH_UP`). El push de prueba manda `test = "true"` y `sent_at`.
- **Prioridad**: Android `high`; iOS `apns-priority: 10` con sonido `default`.

## Configuración

| Variable | Uso |
|---|---|
| `GOOGLE_APPLICATION_CREDENTIALS` | **Ruta** al JSON de la cuenta de servicio de Firebase (Application Default Credentials; el SDK toma de ahí el project ID). Obligatoria: sin ella el módulo no arranca. El JSON nunca va al repo ni a la imagen (`.gitignore`/`.dockerignore`: `*firebase-adminsdk*.json`, `backend/secrets/`). |
| `NOTIFICATIONS_PUSH_TOPIC` | Topic de FCM (default `alerts`). |

Habilitar: `AGATHA_MODULES=…,alert_management,notifications` (requiere `alert_management`). En Docker hay que montar el JSON como volumen de solo lectura y apuntar la variable a la ruta DENTRO del contenedor.

## Aislamiento

Define su propio `AlertNotice`; no importa `alert_management` (ni su `contract/`), otros módulos ni `shared/eventbus` (`depguard` `notifications-isolation`). La traducción del evento y la suscripción viven en `internal/bootstrap/notifications.go`. El SDK de Firebase (`firebase.google.com/go/v4`) solo se usa en `infrastructure/fcm`.

## Estructura

```
domain/entities          AlertNotice, Push (+ Validate)
application/ports        PushSender
application/services     AlertPushService (late → no envía; arma el Push)
infrastructure/events    AlertNotificationSubscriber (registra errores)
infrastructure/fcm       Sender (Admin SDK v4) + toMessage
```
