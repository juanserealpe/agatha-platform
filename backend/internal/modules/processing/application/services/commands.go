package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// Config son los parámetros de Processing (variables PROCESSING_*).
type Config struct {
	MaxSlotsEvent        int           // PROCESSING_MAX_SLOTS_EVENT (24): respeta el timeout de 30 s del bus
	MaxSlotsReconciler   int           // PROCESSING_MAX_SLOTS_RECONCILER (72)
	InitialLookbackHours int           // PROCESSING_INITIAL_LOOKBACK_HOURS (72)
	Grace                time.Duration // PROCESSING_GRACE (30 min): espera antes de evaluar una hora sin datos
	ConfigVersion        string        // "m=<B1>,<B2>,<Planitud>"
	SilentPeriod         time.Duration // fijo: 7 d
}

// DefaultConfig devuelve los valores del spec. ConfigVersion lo arma bootstrap.
func DefaultConfig() Config {
	return Config{
		MaxSlotsEvent:        24,
		MaxSlotsReconciler:   72,
		InitialLookbackHours: 72,
		Grace:                30 * time.Minute,
		SilentPeriod:         7 * 24 * time.Hour,
	}
}

// EvaluatePendingCommand es la entrada de UC-P1.
type EvaluatePendingCommand struct {
	PointID int64
	// ReadyUntil es la última hora lista. Cero ⇒ floorHour(MAX(timestamp) de B2):
	// el evento actual de Ingestion no transporta watermark. Nunca supera floorHour(now).
	ReadyUntil time.Time
	Trigger    entities.Trigger
	BatchSeq   *int64 // solo trazabilidad (source_batch_seq)
	// MaxSlots es el tope de horas por invocación (≤ 0 ⇒ sin tope, solo replay).
	MaxSlots int
	// DataFrom es el menor timestamp del lote (opcional). Si es anterior a la
	// última hora evaluada se registra un Warn de datos tardíos.
	DataFrom *time.Time
}

// EvaluationResult es la salida de UC-P1.
type EvaluationResult struct {
	PointID    int64
	ReadyUntil time.Time
	Planned    int // horas planificadas (cada una produce 2 filas)
	Inserted   int // filas realmente insertadas
	FirstSlot  *time.Time
	LastSlot   *time.Time
}

// PointReconciliation es el resumen de UC-P2 para un punto.
type PointReconciliation struct {
	PointID int64
	Result  EvaluationResult
	Err     error
}
