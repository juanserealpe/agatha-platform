package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

// Errores del Rule Engine. Una hora ignorada (ventana de 6 h, NO_CALCULABLE),
// un duplicado o una llegada tardía NO son errores: son resultados.
var (
	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}

	// Validación de umbrales (mismas reglas que los CHECK de la tabla).
	ErrThresholdOrder = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "los umbrales deben cumplir 0 < amarillo < naranja < rojo <= 1",
	}
	ErrExitMarginRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el margen de salida debe cumplir 0 <= exit_margin < amarillo",
	}
	ErrHoursRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "min_hours y exit_hours deben ser >= 1 y los escalamientos NULL o >= 1",
	}
	ErrInvalidEventType = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "tipo de evento no soportado",
	}
	ErrMissingAuthorship = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "reason y created_by son obligatorios",
	}
	ErrValidFromInPast = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "una versión de umbrales no puede empezar en el pasado (no hay cambios retroactivos)",
	}
	ErrValidFromNotAfterCurrent = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la versión nueva debe empezar después de la versión vigente",
	}
	ErrInvalidTimeRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el rango de tiempo no es válido (from debe ser anterior a to)",
	}
	ErrNoPoints = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "no hay puntos de medición a los que aplicar la versión",
	}

	ErrThresholdsNotFound = &errors.AppError{
		Code:    errors.CodeNotFound,
		Message: "el punto no tiene umbrales configurados",
	}
	ErrConsultingThresholds = &errors.AppError{
		Code:    errors.CodeConsultingThresholds,
		Message: "no fue posible consultar los umbrales del punto de medición",
	}
	ErrPersistingThresholds = &errors.AppError{
		Code:    errors.CodePersistingThresholds,
		Message: "no fue posible guardar la versión de umbrales",
	}

	ErrConsultingRuleEvaluations = &errors.AppError{
		Code:    errors.CodeConsultingRuleEvaluations,
		Message: "no fue posible consultar las evaluaciones de reglas",
	}
	ErrPersistingRuleEvaluations = &errors.AppError{
		Code:    errors.CodePersistingRuleEvaluations,
		Message: "no fue posible guardar la evaluación de reglas",
	}
	ErrConsultingIndicators = &errors.AppError{
		Code:    errors.CodeConsultingEvaluations,
		Message: "no fue posible consultar las evaluaciones del índice (feature_evaluations)",
	}

	ErrRebuildForbidden = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-rebuild no está permitido con AGATHA_ENV=production",
	}
)
