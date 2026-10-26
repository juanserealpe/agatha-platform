package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// CreatePolicyCommand crea una versión de política.
type CreatePolicyCommand struct {
	// PointID nil = versión global; con punto, reemplaza a la global en ese punto.
	PointID   *int64
	EventType entities.EventType
	Patch     entities.PolicyPatch
	// ValidFrom cero ⇒ now. Nunca en el pasado.
	ValidFrom time.Time
	Reason    string
	CreatedBy string
}

// PolicyService versiona alert_policy_versions (solo se agregan versiones, sin retroactividad).
type PolicyService struct {
	store ports.PolicyStore
	clock ports.Clock
	log   *logger.Logger
}

func NewPolicyService(store ports.PolicyStore, clock ports.Clock, log *logger.Logger) *PolicyService {
	return &PolicyService{store: store, clock: clock, log: log}
}

// Current devuelve la versión vigente ahora: la del punto si existe; si no, la global.
func (s *PolicyService) Current(ctx context.Context, pointID *int64, eventType entities.EventType) (entities.Policy, error) {
	eventType = defaultEvent(eventType)
	var p *entities.Policy
	var err error
	if pointID != nil {
		p, err = s.store.GetEffective(ctx, *pointID, eventType, s.clock.Now().UTC())
	} else {
		p, err = s.store.GetEffectiveExact(ctx, nil, eventType, s.clock.Now().UTC())
	}
	if err != nil {
		s.log.Error("[PolicyService] Error consultando la política", "error", err)
		return entities.Policy{}, domain.ErrConsultingAlerts
	}
	if p == nil {
		return entities.Policy{EventType: eventType, PolicyParams: entities.DefaultPolicyParams()}, nil
	}
	return *p, nil
}

// History devuelve las versiones del alcance (punto o global), ascendentes.
func (s *PolicyService) History(ctx context.Context, pointID *int64, eventType entities.EventType) ([]entities.Policy, error) {
	out, err := s.store.History(ctx, pointID, defaultEvent(eventType))
	if err != nil {
		s.log.Error("[PolicyService] Error consultando el historial de políticas", "error", err)
		return nil, domain.ErrConsultingAlerts
	}
	return out, nil
}

// Create copia de la vigente los campos que el parche no informa, valida y
// cierra la vigente del alcance en la misma transacción.
func (s *PolicyService) Create(ctx context.Context, cmd CreatePolicyCommand) (entities.Policy, error) {
	eventType := defaultEvent(cmd.EventType)
	if !eventType.Valid() || (cmd.PointID != nil && *cmd.PointID <= 0) {
		return entities.Policy{}, domain.ErrInvalidPolicy
	}
	if strings.TrimSpace(cmd.Reason) == "" || strings.TrimSpace(cmd.CreatedBy) == "" {
		return entities.Policy{}, domain.ErrMissingAuthorship
	}
	now := s.clock.Now().UTC()
	validFrom := cmd.ValidFrom.UTC()
	if cmd.ValidFrom.IsZero() {
		validFrom = now
	}
	if validFrom.Before(now.Add(-time.Minute)) {
		return entities.Policy{}, domain.ErrValidFromInPast
	}

	base, err := s.Current(ctx, cmd.PointID, eventType)
	if err != nil {
		return entities.Policy{}, err
	}
	params := cmd.Patch.Apply(base.PolicyParams).Normalize()
	if !params.Validate() {
		return entities.Policy{}, domain.ErrInvalidPolicy
	}

	created, err := s.store.Create(ctx, entities.Policy{
		MeasuringPointID: cmd.PointID, EventType: eventType, PolicyParams: params,
		ValidFrom: validFrom, Reason: strings.TrimSpace(cmd.Reason), CreatedBy: strings.TrimSpace(cmd.CreatedBy), CreatedAt: now,
	})
	if errors.Is(err, ports.ErrConflict) {
		return entities.Policy{}, domain.ErrPolicyConflict
	}
	if err != nil {
		s.log.Error("[PolicyService] Error guardando la política", "error", err)
		return entities.Policy{}, domain.ErrPersistingAlerts
	}
	s.log.Info("[PolicyService] Versión de política creada", "id", created.ID, "version", created.Version,
		"measuring_point_id", cmd.PointID, "valid_from", created.ValidFrom, "created_by", created.CreatedBy)
	return created, nil
}

func defaultEvent(e entities.EventType) entities.EventType {
	if e == "" {
		return entities.EventTypeLeak
	}
	return e
}
