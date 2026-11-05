// Package contract es la superficie pública de Alert Management para otros
// módulos (Notification): tópicos y eventos. Solo depende de la stdlib.
//
// Alert Management NO entrega avisos: decide QUÉ mensaje es una notificación
// (catálogo y reglas de la especificación) y lo publica. El canal (web, push,
// correo) y los reintentos son del módulo de notificaciones.
//
// Clave de idempotencia para los consumidores: MessageID. Los eventos son
// best-effort (se publican después del commit): la fuente de verdad es la
// tabla alert_messages (kind = 'NOTIFICACION').
package contract

const (
	// TopicAlertNotification se publica por CADA mensaje NOTIFICACION nuevo de
	// una hora procesada en vivo o por el catch-up, después del commit. No se
	// publica en el backfill ni en reprocesos (el mensaje ya existía).
	TopicAlertNotification = "alert_management.alert_notification"

	// TopicAlertChanged se publica por cada cambio visible de una alerta
	// (AlertChanged), después del commit. Tampoco se publica en el backfill.
	TopicAlertChanged = "alert_management.alert_changed"

	// SchemaVersion es la versión del esquema de los eventos de este contrato.
	SchemaVersion = 1
)

// Source: de dónde vino la hora que generó el aviso.
const (
	SourceLive    = "LIVE"
	SourceCatchUp = "CATCH_UP"
)
