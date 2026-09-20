package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

var (
	ErrInvalidRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "rango de fechas inválido",
	}

	ErrFutureDate = &errors.AppError{
		Code:    CodeFutureDate,
		Message: "la fecha solicitada no puede ser futura",
	}

	ErrBeforeInitialDate = &errors.AppError{
		Code:    CodeBeforeInitialDate,
		Message: "la fecha solicitada es anterior a la fecha inicial de ingesta",
	}

	ErrAlreadyIngested = &errors.AppError{
		Code:    CodeAlreadyIngested,
		Message: "el rango solicitado ya fue ingerido",
	}

	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}

	ErrConsultingLastState = &errors.AppError{
		Code:    CodeConsultingLastState,
		Message: "error consultando el último estado de ingesta",
	}

	ErrGettingIngestedDataInDataOS = &errors.AppError{
		Code:    CodeGettingIngestedData,
		Message: "error consultando los datos desde DataOS",
	}

	ErrRecivingIngestedData = &errors.AppError{
		Code:    CodeGettingIngestedData,
		Message: "No se recibieron mediciones desde DataOS para el rango solicitado",
	}
)

const (
	CodeAlreadyIngested         errors.ErrorCode = "ALREADY_INGESTED"
	CodeFutureDate              errors.ErrorCode = "FUTURE_DATE"
	CodeBeforeInitialDate       errors.ErrorCode = "BEFORE_INITIAL_DATE"
	CodeInvalidMeasuringPointID errors.ErrorCode = "INVALID_MEASURING_POINT_ID"
	CodeConsultingLastState     errors.ErrorCode = "CONSULTING_LAST_STATE"
	CodeGettingIngestedData     errors.ErrorCode = "GETTING_INGESTED_DATA"
)
