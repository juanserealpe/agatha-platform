package services

import (
	"time"

	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestionValidator struct {
	logger *logger.Logger
}

func NewIngestionValidator(
	logger *logger.Logger,
) *IngestionValidator {
	return &IngestionValidator{
		logger: logger,
	}
}

func (s *IngestionValidator) validateRange(start time.Time, end time.Time, now time.Time) error {
	if start.IsZero() || end.IsZero() {
		return appErrors.ErrInvalidDate
	}

	if !start.Before(end) {
		return appErrors.ErrInvalidDate
	}

	if end.After(now) {
		return appErrors.ErrInvalidDate
	}
	return nil
}
