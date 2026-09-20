package services

import (
	"time"

	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
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
		return ingestionErrors.ErrInvalidRange
	}

	if !start.Before(end) {
		return ingestionErrors.ErrInvalidRange
	}

	if end.After(now) {
		return ingestionErrors.ErrFutureDate
	}
	return nil
}
