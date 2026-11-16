// Package memory implementa los repositorios del Rule Engine en memoria, con
// las mismas reglas que el esquema de PostgreSQL (UNIQUE de versión, EXCLUDE de
// solapamientos y rollback de la transacción). Es para tests y pruebas locales:
// el cableado de producción usa infrastructure/persistence/repositories.
package memory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
)

// Errores equivalentes a las restricciones de la tabla (envuelven
// repositories.ErrConflict, como el repositorio de PostgreSQL).
var (
	ErrDuplicateVersion = fmt.Errorf("memory: violación de UNIQUE (measuring_point_id, event_type, version): %w", repositories.ErrConflict)
	ErrOverlap          = fmt.Errorf("memory: violación de EXCLUDE (vigencias solapadas): %w", repositories.ErrConflict)
	ErrNotFound         = errors.New("memory: versión inexistente")
	ErrForeignKey       = errors.New("memory: violación de FK (rule_evaluations referencia la versión)")
)

// ThresholdRepository guarda las versiones en memoria.
type ThresholdRepository struct {
	mu       sync.Mutex
	versions []entities.ThresholdVersion
	nextID   int64
	// FailNextTransaction, si no es nil, hace fallar la próxima Transaction
	// antes de ejecutar fn (simula una caída de la BD).
	FailNextTransaction error
}

func NewThresholdRepository(seed ...entities.ThresholdVersion) *ThresholdRepository {
	r := &ThresholdRepository{}
	for _, v := range seed {
		r.nextID++
		if v.ID == 0 {
			v.ID = r.nextID
		}
		r.versions = append(r.versions, v)
	}
	return r
}

// All devuelve una copia de todas las versiones (para asserts).
func (r *ThresholdRepository) All() []entities.ThresholdVersion {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.versions)
}

func (r *ThresholdRepository) GetEffectiveAt(_ context.Context, pointID int64, eventType entities.EventType, t time.Time) (*entities.ThresholdVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versions {
		if v.MeasuringPointID == pointID && v.EventType == eventType && v.IsEffectiveAt(t) {
			found := v
			return &found, nil
		}
	}
	return nil, nil
}

func (r *ThresholdRepository) GetHistory(_ context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.filter(pointID, eventType)
	slices.SortFunc(out, func(a, b entities.ThresholdVersion) int { return a.Version - b.Version })
	return out, nil
}

func (r *ThresholdRepository) GetOverlapping(_ context.Context, pointID int64, eventType entities.EventType, from, to time.Time) ([]entities.ThresholdVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entities.ThresholdVersion
	for _, v := range r.filter(pointID, eventType) {
		if v.ValidFrom.Before(to) && (v.ValidTo == nil || v.ValidTo.After(from)) {
			out = append(out, v)
		}
	}
	slices.SortFunc(out, func(a, b entities.ThresholdVersion) int { return a.ValidFrom.Compare(b.ValidFrom) })
	return out, nil
}

func (r *ThresholdRepository) ListPoints(_ context.Context, eventType entities.EventType) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var points []int64
	for _, v := range r.versions {
		if v.EventType == eventType && !slices.Contains(points, v.MeasuringPointID) {
			points = append(points, v.MeasuringPointID)
		}
	}
	slices.Sort(points)
	return points, nil
}

// Transaction trabaja sobre una copia y la confirma solo si fn no falla.
func (r *ThresholdRepository) Transaction(ctx context.Context, fn func(tx repositories.ThresholdWriter) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.FailNextTransaction; err != nil {
		r.FailNextTransaction = nil
		return err
	}

	tx := &thresholdTx{versions: slices.Clone(r.versions), nextID: r.nextID}
	if err := fn(tx); err != nil {
		return err
	}

	r.versions, r.nextID = tx.versions, tx.nextID
	return nil
}

func (r *ThresholdRepository) filter(pointID int64, eventType entities.EventType) []entities.ThresholdVersion {
	var out []entities.ThresholdVersion
	for _, v := range r.versions {
		if v.MeasuringPointID == pointID && v.EventType == eventType {
			out = append(out, v)
		}
	}
	return out
}

type thresholdTx struct {
	versions []entities.ThresholdVersion
	nextID   int64
}

func (tx *thresholdTx) LockOpen(_ context.Context, pointID int64, eventType entities.EventType) (*entities.ThresholdVersion, error) {
	for _, v := range tx.versions {
		if v.MeasuringPointID == pointID && v.EventType == eventType && v.ValidTo == nil {
			found := v
			return &found, nil
		}
	}
	return nil, nil
}

func (tx *thresholdTx) Close(_ context.Context, versionID int64, validTo time.Time) error {
	for i := range tx.versions {
		if tx.versions[i].ID == versionID {
			closed := validTo
			tx.versions[i].ValidTo = &closed
			return nil
		}
	}
	return ErrNotFound
}

func (tx *thresholdTx) Insert(_ context.Context, v entities.ThresholdVersion) (int64, error) {
	for _, other := range tx.versions {
		if other.MeasuringPointID != v.MeasuringPointID || other.EventType != v.EventType {
			continue
		}
		if other.Version == v.Version {
			return 0, ErrDuplicateVersion
		}
		if overlaps(other, v) {
			return 0, ErrOverlap
		}
	}

	tx.nextID++
	v.ID = tx.nextID
	tx.versions = append(tx.versions, v)
	return v.ID, nil
}

// overlaps emula tstzrange(valid_from, valid_to) && tstzrange(...) con rangos [).
func overlaps(a, b entities.ThresholdVersion) bool {
	aEndsAfterBStarts := a.ValidTo == nil || a.ValidTo.After(b.ValidFrom)
	bEndsAfterAStarts := b.ValidTo == nil || b.ValidTo.After(a.ValidFrom)
	return aEndsAfterBStarts && bEndsAfterAStarts
}
