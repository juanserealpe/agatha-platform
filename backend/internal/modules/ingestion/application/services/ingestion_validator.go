package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

type IngestionValidator struct {
	deviceRepository repositories.DeviceRepository
}

func NewIngestionValidator(
	deviceRepository repositories.DeviceRepository,
) *IngestionValidator {
	return &IngestionValidator{
		deviceRepository: deviceRepository,
	}
}

func (s *IngestionValidator) validateRange(start time.Time, end time.Time, now time.Time) error {
	fmt.Print("validateRange: start: ", start, " end: ", end, " now: ", now)
	if start.IsZero() || end.IsZero() {
		return appErrors.ErrInvalidRange
	}

	if !start.Before(end) {
		return fmt.Errorf("%w: la fecha de inicio debe ser anterior a la fecha final", appErrors.ErrInvalidRange)
	}

	if end.After(now) {
		return fmt.Errorf("%w: fecha final %s", appErrors.ErrFutureDate, end)
	}
	fmt.Print("validateRange: rango válido")
	return nil
}

func (s *IngestionValidator) validateIngestionRangeHistorical(lastState *entities.IngestionState, start time.Time, end time.Time, now time.Time) error {

	if lastState == nil {
		return nil
	}

	if start.Before(lastState.InitialDate) {
		return fmt.Errorf("%w: inicio %s, fecha inicial %s", appErrors.ErrBeforeInitialDate, start, lastState.InitialDate)
	}

	if lastState.LastIngestedAt != nil && !start.After(*lastState.LastIngestedAt) {
		return fmt.Errorf("%w: inicio %s, última fecha ingerida %s", appErrors.ErrAlreadyIngested, start, *lastState.LastIngestedAt)
	}

	return nil
}

func (s *IngestionValidator) getActiveDevice(ctx context.Context, measuringPointID int64,
) (*entities.Device, error) {

	device, err := s.deviceRepository.FindByMeasuringPointID(ctx, measuringPointID)

	if err != nil {
		return nil, fmt.Errorf("find device %q: %w", measuringPointID, err)
	}

	if device == nil {
		return nil, fmt.Errorf(
			"%w: dispositivo %d", appErrors.ErrNotFound, measuringPointID)
	}

	if !device.Active {
		return nil, fmt.Errorf(
			"%w: dispositivo %d", appErrors.ErrDeviceInactive, device.MeasuringPointID)
	}

	return device, nil
}
