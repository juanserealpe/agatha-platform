package memory

import (
	"context"
	"slices"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
)

// HistoryResetter implementa repositories.HistoryResetter sobre los
// repositorios en memoria, con la FK de rule_evaluations → versión (RESTRICT) y
// el rollback de la transacción. Es para tests: producción usa GORM.
type HistoryResetter struct {
	thresholds  *ThresholdRepository
	evaluations *RuleEvaluationRepository

	// FirstMeasurement simula MIN(raw_measurements.timestamp) por punto.
	FirstMeasurement map[int64]time.Time
	// FailInsert, si no es nil, hace fallar InsertVersion (simula una caída de la BD).
	FailInsert error
}

func NewHistoryResetter(thresholds *ThresholdRepository, evaluations *RuleEvaluationRepository) *HistoryResetter {
	return &HistoryResetter{thresholds: thresholds, evaluations: evaluations}
}

// Transaction toma ambos repositorios, ejecuta fn y, si falla, restaura las
// versiones y las filas como estaban.
func (h *HistoryResetter) Transaction(_ context.Context, fn func(tx repositories.HistoryResetTx) error) error {
	h.thresholds.mu.Lock()
	defer h.thresholds.mu.Unlock()
	h.evaluations.mu.Lock()
	defer h.evaluations.mu.Unlock()

	versions, rows := slices.Clone(h.thresholds.versions), slices.Clone(h.evaluations.rows)
	if err := fn(&historyTx{h: h}); err != nil {
		h.thresholds.versions, h.evaluations.rows = versions, rows
		return err
	}
	return nil
}

type historyTx struct{ h *HistoryResetter }

func (tx *historyTx) LockVersions(_ context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error) {
	out := tx.h.thresholds.filter(pointID, eventType)
	slices.SortFunc(out, func(a, b entities.ThresholdVersion) int { return a.Version - b.Version })
	return out, nil
}

func (tx *historyTx) FirstMeasurementAt(_ context.Context, pointID int64) (*time.Time, error) {
	if at, ok := tx.h.FirstMeasurement[pointID]; ok {
		return &at, nil
	}
	return nil, nil
}

func (tx *historyTx) DeleteEvaluations(_ context.Context, pointID int64, eventType entities.EventType) (int64, error) {
	var kept []entities.RuleEvaluation
	var deleted int64
	for _, e := range tx.h.evaluations.rows {
		if e.MeasuringPointID == pointID && e.EventType == eventType {
			deleted++
			continue
		}
		kept = append(kept, e)
	}
	tx.h.evaluations.rows = kept
	return deleted, nil
}

// DeleteVersions emula la FK RESTRICT: falla si alguna fila de rule_evaluations
// referencia una de las versiones a borrar.
func (tx *historyTx) DeleteVersions(_ context.Context, pointID int64, eventType entities.EventType) (int64, error) {
	doomed := map[int64]bool{}
	var kept []entities.ThresholdVersion
	for _, v := range tx.h.thresholds.versions {
		if v.MeasuringPointID == pointID && v.EventType == eventType {
			doomed[v.ID] = true
			continue
		}
		kept = append(kept, v)
	}
	for _, e := range tx.h.evaluations.rows {
		if doomed[e.ThresholdVersionID] {
			return 0, ErrForeignKey
		}
	}
	tx.h.thresholds.versions = kept
	return int64(len(doomed)), nil
}

func (tx *historyTx) InsertVersion(_ context.Context, v entities.ThresholdVersion) (int64, error) {
	if tx.h.FailInsert != nil {
		return 0, tx.h.FailInsert
	}
	for _, other := range tx.h.thresholds.versions {
		if other.MeasuringPointID == v.MeasuringPointID && other.EventType == v.EventType &&
			(other.Version == v.Version || overlaps(other, v)) {
			return 0, ErrDuplicateVersion
		}
	}
	tx.h.thresholds.nextID++
	v.ID = tx.h.thresholds.nextID
	tx.h.thresholds.versions = append(tx.h.thresholds.versions, v)
	return v.ID, nil
}
