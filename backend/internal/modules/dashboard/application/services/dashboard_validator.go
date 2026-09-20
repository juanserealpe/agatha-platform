package services

import (
	"time"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
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
	if endDate.Before(startDate) {
		s.dashboardLogger.Error("[DashboardService/ValidatorService] Rango de fechas inválido:startDate=%s, endDate=%s",
			startDate.Format(time.RFC3339), endDate.Format(time.RFC3339))
		return appErrors.ErrInvalidDate
	}
	s.dashboardLogger.Info("[DashboardService/ValidatorService] Rango de fechas válido:startDate=%s, endDate=%s",
		startDate.Format(time.RFC3339), endDate.Format(time.RFC3339))
	return nil
}

func (s *DashboardValidator) ValidateFutureDate(date time.Time) error {
	now := time.Now()
	if date.After(now) {
		s.dashboardLogger.Error("[DashboardService/ValidatorService] Fecha futura inválida: date=%s", date.Format(time.RFC3339))
		return appErrors.ErrFutureDate
	}
	s.dashboardLogger.Info("[DashboardService/ValidatorService] Fecha válida: date=%s", date.Format(time.RFC3339))
	return nil
}
