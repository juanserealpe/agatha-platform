package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

// CodePersistingIngestionState identifica el fallo al guardar la fila de
// ingestion_logs. Se declara acá (no en shared/errors/codes.go) para no tocar
// shared; moverlo allá si otro módulo lo necesita.
const CodePersistingIngestionState errors.ErrorCode = "PERSISTING_INGESTION_STATE"

// Códigos del callback de Dataflow Labs que no tienen equivalente en shared.
// El handler HTTP los traduce a 401, 405, 413 y 415.
const (
	CodeUnauthorized         errors.ErrorCode = "UNAUTHORIZED"
	CodeMethodNotAllowed     errors.ErrorCode = "METHOD_NOT_ALLOWED"
	CodePayloadTooLarge      errors.ErrorCode = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMediaType errors.ErrorCode = "UNSUPPORTED_MEDIA_TYPE"
)

// NewInvalidCallbackPayloadError describe por qué se rechazó el payload del
// callback (→ 400). El detalle nunca incluye credenciales.
func NewInvalidCallbackPayloadError(detail string) *errors.AppError {
	return &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "payload del callback inválido: " + detail,
	}
}

var (
	ErrCallbackUnauthorized = &errors.AppError{
		Code:    CodeUnauthorized,
		Message: "credenciales del callback ausentes o inválidas",
	}

	ErrCallbackMethodNotAllowed = &errors.AppError{
		Code:    CodeMethodNotAllowed,
		Message: "el callback solo acepta POST",
	}

	ErrCallbackPayloadTooLarge = &errors.AppError{
		Code:    CodePayloadTooLarge,
		Message: "el cuerpo del callback supera el tamaño máximo permitido",
	}

	ErrCallbackUnsupportedMediaType = &errors.AppError{
		Code:    CodeUnsupportedMediaType,
		Message: "el callback solo acepta Content-Type application/json",
	}

	ErrConsultingDevice = &errors.AppError{
		Code:    errors.CodeConsultingDevices,
		Message: "error consultando el punto de medición",
	}

	ErrPersistingIngestionState = &errors.AppError{
		Code:    CodePersistingIngestionState,
		Message: "error guardando el estado de ingesta",
	}

	ErrBeforeInitialDate = &errors.AppError{
		Code:    errors.CodeBeforeInitialDate,
		Message: "la fecha solicitada es anterior a la fecha inicial de ingesta",
	}

	ErrAlreadyIngested = &errors.AppError{
		Code:    errors.CodeAlreadyIngested,
		Message: "el rango solicitado ya fue ingerido",
	}

	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}

	ErrConsultingLastState = &errors.AppError{
		Code:    errors.CodeConsultingLastState,
		Message: "error consultando el último estado de ingesta",
	}

	ErrGettingIngestedDataInDataOS = &errors.AppError{
		Code:    errors.CodeGettingIngestedData,
		Message: "error consultando los datos desde DataOS",
	}

	ErrRecivingIngestedData = &errors.AppError{
		Code:    errors.CodeGettingIngestedData,
		Message: "No se recibieron mediciones desde DataOS para el rango solicitado",
	}
)
