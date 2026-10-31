# Notifications

Entrega los eventos de Alert Management por dos canales: **web en tiempo real** (SSE: mapa, panel y banner) y **móvil** (push FCM HTTP v1, contrato en `docs/CONTRATO_PUSH_FCM.md`). No decide qué es una alerta ni guarda alertas: eso es de Alert Management. Plan completo y estado en `docs/PLAN.md`.

> En construcción. Hoy existen el modelo, las reglas por canal, las suscripciones y el hub SSE (aún sin endpoint ni cableado); el stream HTTP y el envío FCM llegan en los próximos commits. Mientras no haya canales configurados, el módulo arranca y descarta los avisos (lo advierte en el log).

> **Foto = estado actual de la alerta.** Cuando este README, el plan o el código dicen "foto" (`alert_changed`), se refieren al estado completo de la alerta justo después de un cambio. No son las evidencias fotográficas de la especificación (RN-09).

## Estructura

```
notifications/
├── domain/
│   ├── entities/      # AlertNotice, AlertChange (vista propia de los eventos), PushMessage + enums del contrato FCM
│   └── policies/      # ChannelPolicy (Strategy): MobilePushPolicy, WebPolicy
├── application/
│   ├── ports/         # NoticeChannel, ChangeChannel (los implementa infrastructure)
│   └── services/      # NoticeDispatcher: política + canal
└── infrastructure/
    ├── events/        # AlertNotificationSubscriber, AlertChangedSubscriber
    └── realtime/      # Hub SSE (canal web): ids epoch-seq, buffer de reenvío, heartbeat
```

## Eventos que consume

| Tópico | Suscripción | Canal |
|---|---|---|
| `alert_management.alert_notification` | `notifications.alert_notification.web` | Web: banner, sonido, notificación del navegador |
| `alert_management.alert_notification` | `notifications.alert_notification.push` | Push FCM |
| `alert_management.alert_changed` | `notifications.alert_changed.web` | Web: mapa y panel |

Cada canal es una suscripción propia del bus, con su cola y su worker: si FCM se atrasa, la web no lo nota. La traducción contrato → `AlertNotice`/`AlertChange` y las suscripciones viven en `internal/bootstrap/notifications.go`; este módulo no importa a Alert Management ni al bus (`depguard` `notifications-isolation`).

## Canal web: hub SSE

`realtime.Hub` implementa `NoticeChannel` y `ChangeChannel`. Publica dos eventos SSE con `data` en JSON (`snake_case`, `null` cuando no hay valor, como la API de alertas):

| Evento SSE | Origen | Uso en el frontend |
|---|---|---|
| `alert.notification` | `AlertNotice` | Banner, sonido y notificación del navegador (deduplicar por `message_id`) |
| `alert.changed` | `AlertChange` (estado actual de la alerta) | Mapa y panel |
| `resync` | El hub | Se perdieron eventos: volver a pedir `/active` y `/pending` |

- **Ids**: `<epoch>-<seq>`. El epoch cambia en cada arranque del proceso.
- **Reenvío**: guarda los últimos 256 eventos. Al reconectar con `Last-Event-ID` (o `?last_event_id=`) reenvía lo que faltó; si el id ya salió del buffer, es de otro epoch o no se entiende, manda `resync`.
- **Heartbeat**: `: ping` cada 25 s (no se guarda ni se reenvía).
- **Clientes lentos**: cada cliente tiene un buffer de 32 frames; si se llena, el hub lo desconecta sin frenar a los demás, y el cliente se resincroniza al volver.
- **Tope**: 200 clientes simultáneos por defecto (`ErrTooManyClients`).
- Al apagarse (`Run` termina), cierra todos los clientes.

## Reglas por canal

| Aviso | Push (`MobilePushPolicy`) | Web (`WebPolicy`) |
|---|---|---|
| Alerta nueva o subida en Amarillo, Naranja o Rojo | Sí | Sí |
| Hora atrasada (`late`) | No | Sí, marcada "Atrasado" |
| Falsa alerta | No | Sí |
| Nivel normal o código desconocido | No | Sí |

Mapeo al push (`entities.NewPushMessage`): `alertId` = código de la alerta (`ALR-000142`); niveles `AMARILLO/NARANJA/ROJO` → `YELLOW/ORANGE/RED`; `FUGA/MOVIMIENTO` → `LEAK/MOVEMENT`; `OPEN` y `NEW_AFTER_CLASSIFIED` → `NEW`; subidas y reactivación → `LEVEL_UP`; `pk` sale de `devices.name` (`PK34-470` → `34+470`) o se omite.

## Habilitación

`notifications` en `AGATHA_MODULES`. No exige `alert_management`: sin él no recibe eventos, y así se puede levantar solo en local sin la cadena de schedulers que procesa la base del piloto.

## Riesgos conocidos

- El bus es en memoria: un aviso se pierde si el proceso cae entre el commit y la publicación. La fuente de verdad es `alert_messages` (`kind = NOTIFICACION`).
- Mejora futura: la abscisa debería ser una columna propia del dispositivo, no deducirse del nombre.
