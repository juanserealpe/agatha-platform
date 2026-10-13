package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

var (
	ErrMissingDeviceID = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "debe seleccionar un dispositivo",
	}

	ErrInvalidDeviceID = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el dispositivo seleccionado no es válido",
	}

	ErrMissingStartDate = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "debe indicar una fecha de inicio",
	}

	ErrInvalidStartDate = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la fecha de inicio no tiene un formato válido",
	}

	ErrMissingEndDate = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "debe indicar una fecha de finalización",
	}

	ErrInvalidEndDate = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la fecha de finalización no tiene un formato válido",
	}

	ErrInvalidDateRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la fecha de inicio debe ser anterior a la fecha de finalización",
	}

	ErrStartDateFuture = &errors.AppError{
		Code:    errors.CodeFutureDate,
		Message: "la fecha de inicio no puede ser posterior a la fecha actual",
	}

	ErrEndDateFuture = &errors.AppError{
		Code:    errors.CodeFutureDate,
		Message: "la fecha de finalización no puede ser posterior a la fecha actual",
	}

	ErrConsultingDevices = &errors.AppError{
		Code:    errors.CodeConsultingDevices,
		Message: "no fue posible consultar los dispositivos",
	}

	ErrConsultingMeasurements = &errors.AppError{
		Code:    errors.CodeConsultingMeasurements,
		Message: "no fue posible consultar las mediciones",
	}

	ErrUnknownVariable = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "alguna de las variables solicitadas no existe",
	}

	ErrConsultingSeries = &errors.AppError{
		Code:    errors.CodeConsultingMeasurements,
		Message: "no fue posible consultar las series del dispositivo",
	}
)
