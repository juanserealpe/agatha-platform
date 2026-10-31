package services

import (
	"time"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// IngestionValidator valida los parámetros de la ingesta DataOS.
type IngestionValidator struct {
	logger *logger.Logger
}

func NewIngestionValidator(logger *logger.Logger) *IngestionValidator {
	return &IngestionValidator{logger: logger}
}

// validateRange exige un rango no vacío con start estrictamente antes de end.
func (s *IngestionValidator) validateRange(start time.Time, end time.Time) error {
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		s.logger.Error("rango de fechas inválido", "start", start, "end", end)
		return appErrors.ErrInvalidDate
	}
	return nil
}
