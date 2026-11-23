package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

// NO_CALCULABLE, la ausencia de línea base y un conflicto de unicidad NO son
// errores: son resultados (ver entities.NoCalcReason).
var (
	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}

	ErrConsultingMeasurements = &errors.AppError{
		Code:    errors.CodeConsultingMeasurements,
		Message: "no fue posible consultar las mediciones del punto de medición",
	}

	ErrConsultingBaseline = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "no fue posible consultar la línea base vigente",
	}

	ErrConsultingEvaluations = &errors.AppError{
		Code:    errors.CodeConsultingEvaluations,
		Message: "no fue posible consultar las evaluaciones del punto de medición",
	}

	ErrPersistingEvaluations = &errors.AppError{
		Code:    errors.CodePersistingEvaluations,
		Message: "no fue posible guardar las evaluaciones del punto de medición",
	}

	// ErrRebuildForbidden: -rebuild reemplaza evaluaciones (inmutables en
	// vivo) y no se permite con AGATHA_ENV=production.
	ErrRebuildForbidden = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-rebuild no está permitido con AGATHA_ENV=production",
	}
)
