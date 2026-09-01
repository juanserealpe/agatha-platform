package errors

var (
	ErrNotFound = &AppError{
		Code:    CodeNotFound,
		Message: "recurso no encontrado",
	}

	ErrInvalidRange = &AppError{
		Code:    CodeInvalidRange,
		Message: "rango de fechas inválido",
	}

	ErrFutureDate = &AppError{
		Code:    CodeFutureDate,
		Message: "la fecha solicitada no puede ser futura",
	}

	ErrBeforeInitialDate = &AppError{
		Code:    CodeBeforeInitialDate,
		Message: "la fecha solicitada es anterior a la fecha inicial de ingesta",
	}

	ErrAlreadyIngested = &AppError{
		Code:    CodeAlreadyIngested,
		Message: "el rango solicitado ya fue ingerido",
	}

	ErrDeviceInactive = &AppError{
		Code:    CodeDeviceInactive,
		Message: "el dispositivo se encuentra inactivo",
	}

	ErrInvalidMeasuringPointID = &AppError{
		Code:    CodeInvalidMeasuringID,
		Message: "el punto de medición no es válido",
	}
)
