package errors

var (
	ErrNotFound = &AppError{
		Code:    CodeNotFound,
		Message: "recurso no encontrado",
	}
	ErrDeviceNotFound = &AppError{
		Code:    CodeNotFound,
		Message: "dispositivo no encontrado",
	}
	ErrFutureDate = &AppError{
		Code:    CodeFutureDate,
		Message: "la fecha de inicio no puede ser futura",
	}
	ErrInvalidDate = &AppError{
		Code:    CodeInvalidInput,
		Message: "la fecha solicitada no es válida",
	}
)
