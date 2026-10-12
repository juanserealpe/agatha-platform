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

	// ErrConsultingKnownEvents: sin la lista de eventos conocidos no se calcula
	// (una línea base contaminada es peor que ninguna).
	ErrConsultingKnownEvents = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "no fue posible consultar los eventos conocidos del punto",
	}

	// ErrBaselineHistoryExists: el histórico retroactivo solo se carga en un
	// punto sin versiones (el versionado no es retroactivo).
	ErrBaselineHistoryExists = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el punto ya tiene versiones de línea base; no se puede cargar el histórico",
	}
)
