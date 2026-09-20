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
)
