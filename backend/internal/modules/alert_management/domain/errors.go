package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

// Errores de Alert Management. Una hora duplicada o vieja NO es error: es un resultado.
var (
	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}
	ErrInvalidAlertID = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el id de la alerta no es válido (número o ALR-000123)",
	}
	ErrAlertNotFound = &errors.AppError{
		Code:    errors.CodeNotFound,
		Message: "la alerta no existe",
	}

	// Gestión.
	ErrInvalidOrigin = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "origin debe ser WEB o APP_MOVIL; desde la app, device_identifier es obligatorio",
	}
	ErrInvalidClassification = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "classification debe ser CONFIRMADA o FALSA_ALERTA",
	}
	ErrInvalidLabel = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la etiqueta no pertenece a la lista de la clasificación de la alerta",
	}
	ErrOtherTextRequired = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "con la etiqueta OTRO, other_text es obligatorio (1 a 100 caracteres); con otra etiqueta no se envía",
	}
	ErrInvalidObservation = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la observación debe tener entre 1 y 500 caracteres",
	}
	ErrInvalidClientRecordID = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "client_record_id debe ser un UUID",
	}
	ErrNotClassified = &errors.AppError{
		Code:    errors.CodeConflict,
		Message: "la alerta todavía no está clasificada: primero clasificar, después etiquetar",
	}
	ErrAlreadyClassified = &errors.AppError{
		Code:    errors.CodeConflict,
		Message: "la alerta ya estaba clasificada: prevalece la primera clasificación recibida",
	}
	ErrAlreadyLabeled = &errors.AppError{
		Code:    errors.CodeConflict,
		Message: "la alerta ya tenía etiqueta: prevalece la primera recibida",
	}

	// Consultas.
	ErrInvalidFilter = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "filtros inválidos: fechas dd/mm/aaaa o aaaa-mm-dd, fin no anterior a inicio, y valores de nivel, estado y evento conocidos",
	}
	ErrExportTooLarge = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "La búsqueda tiene más de 10.000 alertas. Ajuste los filtros para exportar.",
	}

	// Política y eventos conocidos.
	ErrInvalidPolicy = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "parámetros de política inválidos (horas >= 1, 0 < trend_delta < 1, intervention_silent_max_level AMARILLO o NARANJA)",
	}
	ErrValidFromInPast = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "una versión de política no puede empezar en el pasado (no hay cambios retroactivos)",
	}
	ErrMissingAuthorship = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "reason y created_by son obligatorios",
	}
	ErrPolicyConflict = &errors.AppError{
		Code:    errors.CodeConflict,
		Message: "otra versión de política se creó a la vez; reintentar",
	}

	ErrInvalidBody = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el cuerpo no es un JSON válido o tiene campos desconocidos",
	}
	ErrInvalidTimeRange = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el rango de tiempo no es válido (from debe ser anterior a to)",
	}
	ErrRebuildForbidden = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-rebuild no está permitido con AGATHA_ENV=production",
	}
	// ErrManagementPresent: guarda del rebuild (§11.2). El punto tiene
	// clasificaciones, etiquetas u observaciones y no se pasó -drop-management.
	ErrManagementPresent = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el punto tiene gestión (clasificación, etiqueta u observaciones): -rebuild no borra nada sin -drop-management",
	}
	ErrDropManagementRequiresRebuild = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-drop-management solo es válido junto con -rebuild",
	}

	// IO.
	ErrConsultingAlerts = &errors.AppError{
		Code:    errors.CodeConsultingAlerts,
		Message: "error consultando las alertas",
	}
	ErrPersistingAlerts = &errors.AppError{
		Code:    errors.CodePersistingAlerts,
		Message: "error guardando las alertas",
	}
	ErrConsultingRuleEvaluations = &errors.AppError{
		Code:    errors.CodeConsultingRuleEvaluations,
		Message: "error consultando rule_evaluations",
	}
)
