package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type DashboardValidator struct {
	dashboardLogger *logger.Logger
}

func NewDashboardValidator(dashboardLogger *logger.Logger) *DashboardValidator {
	return &DashboardValidator{
		dashboardLogger: dashboardLogger,
	}
}

func (s *DashboardValidator) ValidateDateRange(startDate time.Time, endDate time.Time) error {
	if !startDate.Before(endDate) {
		s.dashboardLogger.Error("[DashboardService/ValidatorService] Rango de fechas inválido",
			"startDate", startDate.Format(time.RFC3339), "endDate", endDate.Format(time.RFC3339))
		return domain.ErrInvalidDateRange
	}
	s.dashboardLogger.Info("[DashboardService/ValidatorService] Rango de fechas válido",
		"startDate", startDate.Format(time.RFC3339), "endDate", endDate.Format(time.RFC3339))
	return nil
}

func (s *DashboardValidator) ValidateNotFuture(date time.Time, futureErr error) error {
	if date.After(time.Now()) {
		s.dashboardLogger.Error("[DashboardService/ValidatorService] Fecha futura inválida", "date", date.Format(time.RFC3339))
		return futureErr
	}
	s.dashboardLogger.Info("[DashboardService/ValidatorService] Fecha válida", "date", date.Format(time.RFC3339))
	return nil
}
