package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// RecalculateCommand es la entrada de UC-B1.
type RecalculateCommand struct {
	PointID int64
	AsOf    time.Time
	Trigger entities.Trigger
	DryRun  bool // calcula sin persistir (validación con el analista)
}

// SkippedReason explica por qué no se creó una versión. No son errores.
type SkippedReason string

const (
	SkippedNone                SkippedReason = ""
	SkippedInsufficientHistory SkippedReason = "INSUFFICIENT_HISTORY"
	SkippedDryRun              SkippedReason = "DRY_RUN"
	SkippedConcurrentUpdate    SkippedReason = "CONCURRENT_UPDATE"
)

// RecalculationResult es la salida de UC-B1. Version tiene la línea base
// calculada aunque no se haya persistido (dry-run); ID y número de versión
// solo se asignan si Created = true.
type RecalculationResult struct {
	PointID       int64
	Created       bool
	Version       entities.BaselineVersion
	Confidence    entities.Confidence
	Flags         []entities.Flag
	Estimation    entities.EstimationSummary
	Passes        []entities.PassReport
	SkippedReason SkippedReason
}

// PointRecalculation es el resumen de UC-B2 para un punto.
type PointRecalculation struct {
	PointID int64
	Due     bool
	Trigger entities.Trigger
	Result  *RecalculationResult
	Err     error
}

// EffectiveResult es la salida de UC-B3: la versión vigente (nil si no hay)
// y el primer registro de B2 del dispositivo.
type EffectiveResult struct {
	Version     *entities.BaselineVersion
	FirstSeenAt *time.Time
}
