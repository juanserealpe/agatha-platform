package services

import (
	"context"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// AlertSnapshot es lo que Baseline necesita de una alerta para excluir su
// intervalo. Tipo propio: Baseline no importa alert_management; bootstrap
// traduce alert_management.alert_changed a este tipo.
type AlertSnapshot struct {
	AlertID          int64
	AlertCode        string
	MeasuringPointID int64
	StartedAt        time.Time
	EndedAt          *time.Time // nil ⇒ en curso
}

// SeedResult resume SeedFromAlerts.
type SeedResult struct {
	Read     int // alertas leídas
	Upserted int // períodos insertados o actualizados
	Skipped  int // alertas sin intervalo válido
}

// ExclusionService mantiene baseline_exclusion_periods: copia el intervalo de
// TODAS las alertas (cualquier clasificación, falsas alarmas incluidas: el tubo
// igual estuvo fuera de lo normal). Es la única escritura de esa tabla además
// de la carga manual; ningún rebuild la toca.
type ExclusionService struct {
	exclusions repositories.ExclusionPeriodRepository
	alerts     repositories.AlertIntervalReader
	log        *logger.Logger
}

func NewExclusionService(
	exclusions repositories.ExclusionPeriodRepository,
	alerts repositories.AlertIntervalReader,
	log *logger.Logger,
) *ExclusionService {
	return &ExclusionService{exclusions: exclusions, alerts: alerts, log: log}
}

// RecordAlert guarda (upsert por código) el intervalo de la foto de una alerta.
// Las alertas de prueba de Notifications (id <= 0) se ignoran.
func (s *ExclusionService) RecordAlert(ctx context.Context, a AlertSnapshot) error {
	if a.AlertID <= 0 {
		s.log.Debug("[ExclusionService] Alerta de prueba ignorada", "alert_code", a.AlertCode)
		return nil
	}

	period, ok := alertPeriod(a.MeasuringPointID, a.AlertCode, a.StartedAt, a.EndedAt)
	if !ok {
		s.log.Warn("[ExclusionService] Alerta sin intervalo válido; no se excluye",
			"alert_code", a.AlertCode, "measuring_point_id", a.MeasuringPointID, "started_at", a.StartedAt)
		return domain.ErrInvalidAlertSnapshot
	}

	if err := s.exclusions.UpsertAlert(ctx, period); err != nil {
		s.log.Error("[ExclusionService] Error guardando el período de exclusión",
			"alert_code", a.AlertCode, "measuring_point_id", a.MeasuringPointID, "error", err)
		return domain.ErrPersistingExclusions
	}
	return nil
}

// SeedFromAlerts copia (upsert) el intervalo de todas las alertas actuales.
// Es idempotente: repetirlo solo actualiza. Se corre una vez al adoptar la
// tabla; después la mantiene RecordAlert.
func (s *ExclusionService) SeedFromAlerts(ctx context.Context) (SeedResult, error) {
	var result SeedResult

	periods, err := s.alerts.ListAlertIntervals(ctx)
	if err != nil {
		s.log.Error("[ExclusionService] Error leyendo las alertas", "error", err)
		return result, domain.ErrConsultingAlerts
	}
	result.Read = len(periods)

	for _, p := range periods {
		period, ok := alertPeriod(p.MeasuringPointID, p.SourceAlertCode, p.StartedAt, p.EndedAt)
		if !ok {
			s.log.Warn("[ExclusionService] Alerta sin intervalo válido; se omite",
				"alert_code", p.SourceAlertCode, "measuring_point_id", p.MeasuringPointID)
			result.Skipped++
			continue
		}
		if err := s.exclusions.UpsertAlert(ctx, period); err != nil {
			s.log.Error("[ExclusionService] Error guardando el período de exclusión",
				"alert_code", p.SourceAlertCode, "error", err)
			return result, domain.ErrPersistingExclusions
		}
		result.Upserted++
	}

	s.log.Info("[ExclusionService] Períodos de exclusión copiados de alerts",
		"read", result.Read, "upserted", result.Upserted, "skipped", result.Skipped)
	return result, nil
}

// alertPeriod arma el período (en UTC) de una alerta; ok = false si el
// intervalo no es válido.
func alertPeriod(pointID int64, code string, started time.Time, ended *time.Time) (entities.ExclusionPeriod, bool) {
	code = strings.TrimSpace(code)
	if pointID <= 0 || code == "" || started.IsZero() || (ended != nil && ended.Before(started)) {
		return entities.ExclusionPeriod{}, false
	}

	period := entities.ExclusionPeriod{
		MeasuringPointID: pointID,
		StartedAt:        started.UTC(),
		Source:           entities.ExclusionSourceAlert,
		SourceAlertCode:  code,
	}
	if ended != nil {
		end := ended.UTC()
		period.EndedAt = &end
	}
	return period, true
}
