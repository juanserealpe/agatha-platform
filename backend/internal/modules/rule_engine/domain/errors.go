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
	ErrPlanitudConfig = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "d_pl_min debe estar entre 0 y 100 y es obligatorio con confirm_naranja_with_planitud",
	}
	ErrRojoPlanitudConfig = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "confirm_rojo_with_planitud exige confirm_naranja_with_planitud",
	}
	ErrPersistenceConfig = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "persistence_mode debe ser STRICT o WINDOW; con WINDOW, persistence_window_hours y persistence_min_hours son obligatorios, >= 1 y min <= ventana",
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
	ErrSimulationRangeTooLong = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el rango de la simulación no puede superar 366 días",
	}
	ErrInvalidBody = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el cuerpo no es un JSON válido o tiene campos desconocidos",
	}
	ErrInvalidPatch = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el patch tiene un campo desconocido, un tipo inválido o un null no permitido",
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
	// ErrThresholdConflict: otra versión se creó a la vez (lock ocupado, UNIQUE
	// o EXCLUDE). El cliente debe reintentar.
	ErrThresholdConflict = &errors.AppError{
		Code:    errors.CodeConflict,
		Message: "otra versión de umbrales se está creando para el mismo punto; reintente",
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

	// Movimiento (motion_threshold_versions y motion_evaluations).
	ErrMotionTempMode = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "temp_mode debe ser MOVIMIENTO_Y_TEMPERATURA o SOLO_MOVIMIENTO",
	}
	ErrMotionJumpShift = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "los umbrales de movimiento deben cumplir 0 < jump_deg <= shift_deg",
	}
	ErrMotionReturnFraction = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "return_fraction debe cumplir 0 < return_fraction <= 1",
	}
	ErrMotionHoursRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "las horas, los minutos y los registros de movimiento deben ser >= 1",
	}
	ErrMotionDegreesRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "los grados y las tasas de temperatura deben ser > 0",
	}
	ErrMotionRecordsRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "min_records_hour debe ser <= records_per_hour",
	}
	ErrMotionThresholdsNotFound = &errors.AppError{
		Code:    errors.CodeNotFound,
		Message: "el punto no tiene umbrales de movimiento configurados",
	}
	ErrConsultingMotionEvaluations = &errors.AppError{
		Code:    errors.CodeConsultingRuleEvaluations,
		Message: "no fue posible consultar las evaluaciones de movimiento",
	}
	ErrPersistingMotionEvaluations = &errors.AppError{
		Code:    errors.CodePersistingRuleEvaluations,
		Message: "no fue posible guardar la evaluación de movimiento",
	}
	ErrConsultingRawMeasurements = &errors.AppError{
		Code:    errors.CodeConsultingEvaluations,
		Message: "no fue posible consultar las mediciones crudas (raw_measurements)",
	}

	ErrRebuildForbidden = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-rebuild y -reset-versions no están permitidos con AGATHA_ENV=production",
	}

	// -reset-versions: guardas de la reconstrucción del historial de versiones
	// de UN punto (segunda excepción deliberada a la no retroactividad).
	ErrResetRequiresRebuild = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-reset-versions solo es válido junto con -backfill -rebuild",
	}
	ErrResetRequiresPoint = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-reset-versions exige -point: no se reconstruyen todos los puntos a la vez",
	}
	ErrResetNotConfirmed = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-reset-versions exige -confirm <point_id> igual al valor de -point",
	}
	ErrResetFullHistory = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-reset-versions reconstruye todo el historial del punto: no admite -from ni -to",
	}
	ErrResetFutureVersions = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el punto tiene versiones programadas a futuro: no se reconstruye el historial para no descartarlas",
	}
	ErrResetUnavailable = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-reset-versions no está disponible: falta el repositorio de reconstrucción de versiones",
	}
)
