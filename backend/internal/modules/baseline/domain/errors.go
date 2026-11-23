package domain

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"

var (
	ErrInvalidMeasuringPointID = &errors.AppError{
		Code:    errors.CodeInvalidMeasuringPointID,
		Message: "el punto de medición no es válido",
	}

	ErrConsultingHistory = &errors.AppError{
		Code:    errors.CodeConsultingMeasurements,
		Message: "no fue posible consultar el histórico de mediciones",
	}

	ErrConsultingBaseline = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "no fue posible consultar la línea base",
	}

	ErrPersistingBaseline = &errors.AppError{
		Code:    errors.CodePersistingBaseline,
		Message: "no fue posible guardar la versión de la línea base",
	}

	// ErrBaselineConcurrentUpdate: otro proceso creó la versión vigente. Se trata como no-op.
	ErrBaselineConcurrentUpdate = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "otro proceso creó la versión vigente de la línea base",
	}

	// ErrConsultingKnownEvents: sin la lista de eventos conocidos no se calcula
	// (una línea base contaminada es peor que ninguna).
	ErrConsultingKnownEvents = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "no fue posible consultar los eventos conocidos del punto",
	}

	// ErrBaselineHistoryExists: el histórico retroactivo solo se carga en un
	// punto sin versiones (el versionado no es retroactivo).
	ErrBaselineHistoryExists = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el punto ya tiene versiones de línea base; no se puede cargar el histórico",
	}

	// ErrRebuildForbidden: -rebuild reemplaza el histórico y no se permite con
	// AGATHA_ENV=production.
	ErrRebuildForbidden = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "-rebuild no está permitido con AGATHA_ENV=production",
	}

	// ErrInvalidAlertSnapshot: la foto de la alerta no sirve como período de
	// exclusión (sin código, punto inválido, sin inicio o fin antes del inicio).
	// ErrActiveAlertPresent: -rebuild no se permite con una alerta vigente en
	// el punto (período ALERT sin ended_at). Se espera a que finalice; no hay
	// flag que lo saltee. Es la misma guarda que aplica Alert Management, pero
	// al principio del runbook, antes de reescribir nada.
	ErrActiveAlertPresent = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "el punto tiene una alerta vigente (sin fecha de fin): -rebuild no se permite hasta que finalice",
	}

	ErrInvalidAlertSnapshot = &errors.AppError{
		Code:    errors.CodeInvalidInput,
		Message: "la alerta no tiene un intervalo válido para excluir de la línea base",
	}

	ErrConsultingAlerts = &errors.AppError{
		Code:    errors.CodeConsultingBaseline,
		Message: "no fue posible consultar las alertas para copiar sus períodos",
	}

	ErrPersistingExclusions = &errors.AppError{
		Code:    errors.CodePersistingBaseline,
		Message: "no fue posible guardar el período de exclusión de la línea base",
	}
)
