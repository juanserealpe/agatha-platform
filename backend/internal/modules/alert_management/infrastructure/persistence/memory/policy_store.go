package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// PolicyStore son las versiones de política en memoria.
type PolicyStore struct {
	mu       sync.Mutex
	versions []entities.Policy
	seq      int64
}

// NewPolicyStore arranca con las versiones dadas (o la global inicial desde
// el año 2000 si no se pasa ninguna).
func NewPolicyStore(versions ...entities.Policy) *PolicyStore {
	if len(versions) == 0 {
		versions = []entities.Policy{{
			ID: 1, Version: 1, EventType: entities.EventTypeLeak, PolicyParams: entities.DefaultPolicyParams(),
			ValidFrom: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Reason: entities.PolicySeedReason, CreatedBy: entities.SystemCreatedBy,
		}}
	}
	s := &PolicyStore{versions: versions}
	for _, v := range versions {
		if v.ID > s.seq {
			s.seq = v.ID
		}
	}
	return s
}

func sameScope(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func effective(p entities.Policy, t time.Time) bool {
	return !p.ValidFrom.After(t) && (p.ValidTo == nil || p.ValidTo.After(t))
}

func (s *PolicyStore) GetEffective(ctx context.Context, pointID int64, eventType entities.EventType, t time.Time) (*entities.Policy, error) {
	if p, _ := s.GetEffectiveExact(ctx, &pointID, eventType, t); p != nil {
		return p, nil
	}
	return s.GetEffectiveExact(ctx, nil, eventType, t)
}

func (s *PolicyStore) GetEffectiveExact(_ context.Context, pointID *int64, eventType entities.EventType, t time.Time) (*entities.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.versions {
		if sameScope(p.MeasuringPointID, pointID) && p.EventType == eventType && effective(p, t) {
			out := p
			return &out, nil
		}
	}
	return nil, nil
}

func (s *PolicyStore) History(_ context.Context, pointID *int64, eventType entities.EventType) ([]entities.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []entities.Policy
	for _, p := range s.versions {
		if sameScope(p.MeasuringPointID, pointID) && p.EventType == eventType {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (s *PolicyStore) Create(_ context.Context, p entities.Policy) (entities.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := 0
	for i := range s.versions {
		v := &s.versions[i]
		if !sameScope(v.MeasuringPointID, p.MeasuringPointID) || v.EventType != p.EventType {
			continue
		}
		version = max(version, v.Version)
		if v.ValidTo == nil {
			if !p.ValidFrom.After(v.ValidFrom) {
				return entities.Policy{}, ports.ErrConflict
			}
			to := p.ValidFrom
			v.ValidTo = &to
		}
	}
	s.seq++
	p.ID = s.seq
	p.Version = version + 1
	s.versions = append(s.versions, p)
	return p, nil
}
