package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

var (
	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}

	ErrConsultingHistory = &errors.AppError{
		Code:    errors.CodeConsultingMeasurements,
		Message: "no fue posible consultar el histórico de mediciones",
	}

	ErrConsultingBaseline = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "no fue posible consultar la línea base",
	}

	ErrPersistingBaseline = &errors.AppError{
		Code:    errors.CodePersistingBaseline,
		Message: "no fue posible guardar la versión de la línea base",
	}

	// ErrBaselineConcurrentUpdate: otro proceso creó la versión vigente. Se trata como no-op.
	ErrBaselineConcurrentUpdate = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "otro proceso creó la versión vigente de la línea base",
	}
)
