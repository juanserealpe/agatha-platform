// Package repositories define la persistencia de las tablas propias del Rule
// Engine (rule_threshold_versions y rule_evaluations).
package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// ErrConflict lo devuelve Transaction (envuelto) cuando otra transacción
// crea una versión del mismo punto a la vez: el lock de la vigente está
// tomado, o se violó el UNIQUE de versión o el EXCLUDE de vigencias. El caso
// de uso responde 409 y el cliente reintenta.
var ErrConflict = errors.New("conflicto de concurrencia en rule_threshold_versions")

// ThresholdRepository lee las versiones de umbrales. Las escrituras solo
// ocurren dentro de Transaction: cerrar la vigente e insertar la nueva van
// SIEMPRE juntas.
type ThresholdRepository interface {
	// GetEffectiveAt devuelve la versión que aplica a la hora t
	// (valid_from <= t AND (valid_to IS NULL OR valid_to > t)); nil si ninguna.
	GetEffectiveAt(ctx context.Context, pointID int64, eventType entities.EventType, t time.Time) (*entities.ThresholdVersion, error)

	// GetHistory devuelve todas las versiones del punto, en orden ascendente de versión.
	GetHistory(ctx context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error)

	// GetOverlapping devuelve, ascendentes por valid_from, las versiones que se
	// solapan con [from, to): valid_from < to AND (valid_to IS NULL OR valid_to > from).
	GetOverlapping(ctx context.Context, pointID int64, eventType entities.EventType, from, to time.Time) ([]entities.ThresholdVersion, error)

	// ListPoints devuelve los puntos que tienen al menos una versión.
	ListPoints(ctx context.Context, eventType entities.EventType) ([]int64, error)

	// Transaction ejecuta fn en UNA transacción: si fn devuelve error no queda nada escrito.
	Transaction(ctx context.Context, fn func(tx ThresholdWriter) error) error
}

// ThresholdWriter es el acceso de escritura ligado a una transacción.
type ThresholdWriter interface {
	// LockOpen devuelve la versión abierta (valid_to IS NULL) bloqueándola hasta
	// el fin de la transacción (SELECT ... FOR UPDATE NOWAIT: si otra
	// transacción la tiene, falla con ErrConflict en vez de esperar). nil si el punto no tiene.
	LockOpen(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.ThresholdVersion, error)

	// Close cierra una versión: valid_to = validTo. Es la ÚNICA modificación
	// permitida sobre una versión existente.
	Close(ctx context.Context, versionID int64, validTo time.Time) error

	// Insert crea la versión y devuelve su ID.
	Insert(ctx context.Context, v entities.ThresholdVersion) (int64, error)
}
