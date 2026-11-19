package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
)

// MotionThresholdRepository es motion_threshold_versions en memoria. La
// transacción es todo o nada: se trabaja sobre una copia.
type MotionThresholdRepository struct {
	mu       sync.Mutex
	versions []entities.MotionThresholdVersion
	nextID   int64
}

func NewMotionThresholdRepository(seed ...entities.MotionThresholdVersion) *MotionThresholdRepository {
	r := &MotionThresholdRepository{nextID: 1}
	for _, v := range seed {
		if v.ID == 0 {
			v.ID = r.nextID
		}
		if v.ID >= r.nextID {
			r.nextID = v.ID + 1
		}
		r.versions = append(r.versions, v)
	}
	return r
}

func (r *MotionThresholdRepository) GetEffectiveAt(_ context.Context, pointID int64, t time.Time) (*entities.MotionThresholdVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versions {
		if v.MeasuringPointID == pointID && v.IsEffectiveAt(t) {
			out := v
			return &out, nil
		}
	}
	return nil, nil
}

func (r *MotionThresholdRepository) GetHistory(_ context.Context, pointID int64) ([]entities.MotionThresholdVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entities.MotionThresholdVersion
	for _, v := range r.versions {
		if v.MeasuringPointID == pointID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (r *MotionThresholdRepository) ListPoints(_ context.Context) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[int64]bool{}
	var out []int64
	for _, v := range r.versions {
		if !seen[v.MeasuringPointID] {
			seen[v.MeasuringPointID] = true
			out = append(out, v.MeasuringPointID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (r *MotionThresholdRepository) Transaction(_ context.Context, fn func(tx repositories.MotionThresholdWriter) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := &motionThresholdWriter{versions: append([]entities.MotionThresholdVersion(nil), r.versions...), nextID: r.nextID}
	if err := fn(w); err != nil {
		return err
	}
	r.versions, r.nextID = w.versions, w.nextID
	return nil
}

type motionThresholdWriter struct {
	versions []entities.MotionThresholdVersion
	nextID   int64
}

func (w *motionThresholdWriter) LockOpen(_ context.Context, pointID int64) (*entities.MotionThresholdVersion, error) {
	for _, v := range w.versions {
		if v.MeasuringPointID == pointID && v.ValidTo == nil {
			out := v
			return &out, nil
		}
	}
	return nil, nil
}

func (w *motionThresholdWriter) Close(_ context.Context, versionID int64, validTo time.Time) error {
	for i := range w.versions {
		if w.versions[i].ID == versionID && w.versions[i].ValidTo == nil {
			t := validTo.UTC()
			w.versions[i].ValidTo = &t
			return nil
		}
	}
	return repositories.ErrConflict
}

func (w *motionThresholdWriter) Insert(_ context.Context, v entities.MotionThresholdVersion) (int64, error) {
	for _, x := range w.versions {
		if x.MeasuringPointID == v.MeasuringPointID && x.Version == v.Version {
			return 0, repositories.ErrConflict
		}
	}
	v.ID = w.nextID
	w.nextID++
	w.versions = append(w.versions, v)
	return v.ID, nil
}

// MotionEvaluationRepository es motion_evaluations en memoria.
type MotionEvaluationRepository struct {
	mu     sync.Mutex
	rows   map[int64][]entities.MotionEvaluation // por punto, ascendentes
	nextID int64
}

func NewMotionEvaluationRepository() *MotionEvaluationRepository {
	return &MotionEvaluationRepository{rows: map[int64][]entities.MotionEvaluation{}, nextID: 1}
}

func (r *MotionEvaluationRepository) GetLastBefore(_ context.Context, pointID int64, before time.Time) (*entities.MotionEvaluation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := r.rows[pointID]
	for i := len(rows) - 1; i >= 0; i-- {
		if before.IsZero() || rows[i].EvaluatedAt.Before(before) {
			out := rows[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (r *MotionEvaluationRepository) Save(_ context.Context, e entities.MotionEvaluation) (int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := r.rows[e.MeasuringPointID]
	for _, x := range rows {
		if x.EvaluatedAt.Equal(e.EvaluatedAt) {
			return 0, false, nil
		}
	}
	e.ID = r.nextID
	r.nextID++
	rows = append(rows, e)
	sort.Slice(rows, func(i, j int) bool { return rows[i].EvaluatedAt.Before(rows[j].EvaluatedAt) })
	r.rows[e.MeasuringPointID] = rows
	return e.ID, true, nil
}

func (r *MotionEvaluationRepository) DeleteRange(_ context.Context, pointID int64, from, to time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var kept []entities.MotionEvaluation
	var n int64
	for _, x := range r.rows[pointID] {
		in := (from.IsZero() || !x.EvaluatedAt.Before(from)) && (to.IsZero() || x.EvaluatedAt.Before(to))
		if in {
			n++
			continue
		}
		kept = append(kept, x)
	}
	r.rows[pointID] = kept
	return n, nil
}

func (r *MotionEvaluationRepository) CountAfter(_ context.Context, pointID int64, at time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, x := range r.rows[pointID] {
		if !x.EvaluatedAt.Before(at) {
			n++
		}
	}
	return n, nil
}

// All devuelve las filas del punto, ascendentes (para tests).
func (r *MotionEvaluationRepository) All(pointID int64) []entities.MotionEvaluation {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]entities.MotionEvaluation(nil), r.rows[pointID]...)
}
