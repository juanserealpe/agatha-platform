package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

var (
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
