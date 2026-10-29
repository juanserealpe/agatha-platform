package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// RuleEvaluationRepository guarda rule_evaluations en memoria, con la misma
// unicidad (punto, evento, hora) que la tabla.
type RuleEvaluationRepository struct {
	mu     sync.Mutex
	rows   []entities.RuleEvaluation
	nextID int64
	// FailSave, si no es nil, hace fallar todos los Save.
	FailSave error
}

func NewRuleEvaluationRepository() *RuleEvaluationRepository {
	return &RuleEvaluationRepository{}
}

// Rows devuelve las filas del punto en orden ascendente de evaluated_at.
func (r *RuleEvaluationRepository) Rows(pointID int64) []entities.RuleEvaluation {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entities.RuleEvaluation
	for _, e := range r.rows {
		if e.MeasuringPointID == pointID {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b entities.RuleEvaluation) int { return a.EvaluatedAt.Compare(b.EvaluatedAt) })
	return out
}

func (r *RuleEvaluationRepository) GetPosition(_ context.Context, pointID int64, eventType entities.EventType, at time.Time) (bool, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	exists, later := false, false
	for _, e := range r.rows {
		if e.MeasuringPointID != pointID || e.EventType != eventType {
			continue
		}
		exists = exists || e.EvaluatedAt.Equal(at)
		later = later || e.EvaluatedAt.After(at)
	}
	return exists, later, nil
}

func (r *RuleEvaluationRepository) GetPrevious(_ context.Context, pointID int64, eventType entities.EventType, before time.Time, limit int) ([]entities.RuleEvaluation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entities.RuleEvaluation
	for _, e := range r.rows {
		if e.MeasuringPointID == pointID && e.EventType == eventType && e.EvaluatedAt.Before(before) {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b entities.RuleEvaluation) int { return b.EvaluatedAt.Compare(a.EvaluatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *RuleEvaluationRepository) Save(_ context.Context, e entities.RuleEvaluation) (int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.FailSave != nil {
		return 0, false, r.FailSave
	}
	for _, other := range r.rows {
		if other.MeasuringPointID == e.MeasuringPointID && other.EventType == e.EventType && other.EvaluatedAt.Equal(e.EvaluatedAt) {
			return 0, false, nil
		}
	}
	r.nextID++
	e.ID = r.nextID
	r.rows = append(r.rows, e)
	return e.ID, true, nil
}

func (r *RuleEvaluationRepository) DeleteRange(_ context.Context, pointID *int64, eventType entities.EventType, from, to time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.rows[:0]
	var deleted int64
	for _, e := range r.rows {
		match := e.EventType == eventType &&
			(pointID == nil || e.MeasuringPointID == *pointID) &&
			(from.IsZero() || !e.EvaluatedAt.Before(from)) &&
			(to.IsZero() || e.EvaluatedAt.Before(to))
		if match {
			deleted++
			continue
		}
		kept = append(kept, e)
	}
	r.rows = kept
	return deleted, nil
}
