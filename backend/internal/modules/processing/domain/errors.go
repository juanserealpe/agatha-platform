package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

var (
	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición del evento no es válido",
	}

	ErrConsultingLatestMeasurements = &errors.AppError{
		Code:    errors.CodeConsultingMeasurements,
		Message: "no fue posible consultar las mediciones de la última ingesta",
	}

	ErrLatestMeasurementsNotFound = &errors.AppError{
		Code:    errors.CodeNotFound,
		Message: "no se encontraron mediciones para el punto de medición",
	}
)
