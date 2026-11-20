// Package ports son las dependencias de los casos de uso de Alert Management.
// Las implementan infrastructure/persistence (GORM y memoria), export y, para
// NotificationPublisher, el adaptador al bus que vive en bootstrap.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// ErrNotFound lo devuelven los repositorios cuando la fila no existe.
var ErrNotFound = errors.New("alert_management: no encontrado")

// ErrConflict lo devuelve PolicyStore.Create cuando otra versión se crea a la vez.
var ErrConflict = errors.New("alert_management: conflicto de concurrencia")

// Clock da la hora actual (inyectable en pruebas).
type Clock interface {
	Now() time.Time
}

// ClockFunc adapta una función a Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

// AlertStore abre las transacciones de escritura de alertas.
type AlertStore interface {
	Transaction(ctx context.Context, fn func(tx AlertTx) error) error
}

// AlertTx es el acceso de escritura ligado a una transacción.
type AlertTx interface {
	GetState(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.ProcessingState, error)
	SaveState(ctx context.Context, s entities.ProcessingState) error

	// GetActive devuelve la alerta ACTIVA del (punto, evento), nil si no hay.
	// FUGA y MOVIMIENTO nunca comparten alerta: cada uno tiene la suya.
	GetActive(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.Alert, error)
	// GetLastFinished devuelve la última alerta FINALIZADA del (punto, evento)
	// (la de ended_at más reciente), nil si no hay.
	GetLastFinished(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.Alert, error)
	// GetAlert devuelve la alerta (ErrNotFound si no existe). forUpdate la bloquea hasta el fin de la TX.
	GetAlert(ctx context.Context, id int64, forUpdate bool) (*entities.Alert, error)
	// InsertAlert crea la alerta y devuelve su ID.
	InsertAlert(ctx context.Context, a entities.Alert) (int64, error)
	// UpdateProcessing escribe SOLO las columnas del procesamiento horario
	// (nunca las de gestión, que son inmutables y las escribe otro caso de uso).
	UpdateProcessing(ctx context.Context, a entities.Alert) error

	// GetCurrentSegment devuelve el color vigente o, si la alerta finalizó, el último.
	GetCurrentSegment(ctx context.Context, alertID int64) (*entities.Segment, error)
	// InsertSegment crea un color con Seq = último + 1 y devuelve su ID.
	InsertSegment(ctx context.Context, s entities.Segment) (int64, error)
	CloseSegment(ctx context.Context, segmentID int64, at time.Time) error

	// InsertMessage agrega un mensaje (ON CONFLICT (alert_id, evaluated_at, code)
	// DO NOTHING: reprocesar no duplica). inserted = false ⇒ ya existía.
	InsertMessage(ctx context.Context, m entities.Message) (id int64, inserted bool, err error)

	// Classify escribe la clasificación con UPDATE … WHERE classification IS
	// NULL. updated = false ⇒ ya estaba clasificada (conflicto).
	Classify(ctx context.Context, alertID int64, m entities.Management) (updated bool, err error)
	// Label escribe la etiqueta con UPDATE … WHERE classification IS NOT NULL
	// AND label IS NULL. updated = false ⇒ ya tenía etiqueta (conflicto).
	Label(ctx context.Context, alertID int64, m entities.Management) (updated bool, err error)
	// InsertObservation agrega una observación (ON CONFLICT client_record_id DO NOTHING).
	InsertObservation(ctx context.Context, o entities.Observation) (id int64, inserted bool, err error)
	// ClientRecordSeen indica si el client_record_id ya se recibió (en la
	// clasificación, la etiqueta o una observación).
	ClientRecordSeen(ctx context.Context, clientRecordID string) (bool, error)
}

// PolicyStore lee y versiona alert_policy_versions.
type PolicyStore interface {
	// GetEffective devuelve la versión vigente en t: la del punto si existe; si no, la global.
	GetEffective(ctx context.Context, pointID int64, eventType entities.EventType, t time.Time) (*entities.Policy, error)
	// GetEffectiveExact devuelve la versión vigente en t con ese alcance exacto (pointID nil = global).
	GetEffectiveExact(ctx context.Context, pointID *int64, eventType entities.EventType, t time.Time) (*entities.Policy, error)
	// History devuelve las versiones del alcance, ascendentes.
	History(ctx context.Context, pointID *int64, eventType entities.EventType) ([]entities.Policy, error)
	// Create cierra la vigente del alcance (valid_to = p.ValidFrom) e inserta p en una TX.
	Create(ctx context.Context, p entities.Policy) (entities.Policy, error)
}

// DeviceDirectory lee los dispositivos (tabla devices de Ingestion, solo lectura).
type DeviceDirectory interface {
	Get(ctx context.Context, pointID int64) (*entities.Device, error)
}

// LevelSource lee rule_evaluations (del Rule Engine, solo lectura) para el
// catch-up y el backfill.
type LevelSource interface {
	// ListPoints devuelve los puntos con alguna fila.
	ListPoints(ctx context.Context) ([]int64, error)
	// ListAfter devuelve hasta limit horas del punto con evaluated_at > after
	// (cero = desde el inicio) y < to (cero = sin límite), ascendentes.
	ListAfter(ctx context.Context, pointID int64, after, to time.Time, limit int) ([]entities.LevelNotice, error)
}

// ResetCounts es lo que el rebuild va a borrar de un punto (§11.2).
type ResetCounts struct {
	Alerts, Messages, Classified, Labeled, Observations int64
}

// HasManagement indica si alguna alerta del punto tiene gestión.
func (c ResetCounts) HasManagement() bool {
	return c.Classified > 0 || c.Labeled > 0 || c.Observations > 0
}

// AlertReset borra los datos de Alert Management de un punto (solo -rebuild fuera de producción).
type AlertReset interface {
	// CountPoint cuenta lo que ResetPoint borraría.
	CountPoint(ctx context.Context, pointID int64) (ResetCounts, error)
	// ResetPoint borra TODAS las alertas del punto (de los dos flujos, con
	// colores, mensajes, observaciones y gestión) y sus estados de procesamiento.
	ResetPoint(ctx context.Context, pointID int64) (int64, error)
}

// MotionSource lee motion_evaluations (del Rule Engine, solo lectura) para
// el catch-up, el backfill y los huecos de la suscripción. Solo devuelve
// horas de versiones con alerts_enabled (el modo sombra no llega a Alert Management).
type MotionSource interface {
	// ListPoints devuelve los puntos con alguna hora con alerts_enabled.
	ListPoints(ctx context.Context) ([]int64, error)
	// ListAfter devuelve hasta limit horas del punto con evaluated_at > after
	// (cero = desde el inicio) y < to (cero = sin límite), ascendentes.
	ListAfter(ctx context.Context, pointID int64, after, to time.Time, limit int) ([]entities.MotionNotice, error)
}

// NotificationPublisher publica los mensajes NOTIFICACION para el módulo de
// notificaciones. Se llama después del commit: un error no revierte la alerta.
type NotificationPublisher interface {
	PublishAlertNotification(ctx context.Context, n contract.AlertNotification) error
}

// AlertChangePublisher publica la foto de una alerta tras un cambio visible
// (mapa y panel en tiempo real). Se llama después del commit: un error no
// revierte la alerta.
type AlertChangePublisher interface {
	PublishAlertChanged(ctx context.Context, c contract.AlertChanged) error
}
