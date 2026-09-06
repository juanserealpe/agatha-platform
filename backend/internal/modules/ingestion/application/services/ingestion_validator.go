package services

import (
	"context"
	"fmt"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestionValidator struct {
	deviceRepository repositories.DeviceRepository
	logger           *logger.Logger
}

func NewIngestionValidator(
	deviceRepository repositories.DeviceRepository,
	logger *logger.Logger,
) *IngestionValidator {
	return &IngestionValidator{
		deviceRepository: deviceRepository,
		logger:           logger,
	}
}

func (s *IngestionValidator) validateRange(start time.Time, end time.Time, now time.Time) error {
	fmt.Println("Validando el rango de fechas: start: ", start, " end: ", end, " now: ", now)
	if start.IsZero() || end.IsZero() {
		return fmt.Errorf("%w: 2.1 La fecha de inicio o final es inválida", appErrors.ErrInvalidRange)
	}

	if !start.Before(end) {
		return fmt.Errorf("%w: 2.2 la fecha de inicio debe ser anterior a la fecha final", appErrors.ErrInvalidRange)
	}

	if end.After(now) {
		return fmt.Errorf("%w: 2.3 fecha final %s", appErrors.ErrFutureDate, end)
	}
	fmt.Print("2. Validación de rango de fechas exitosa ")
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
