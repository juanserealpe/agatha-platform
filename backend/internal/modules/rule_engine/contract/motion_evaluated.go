package contract

import "time"

const (
	// TopicMotionEvaluated se publica por cada punto y hora de movimiento
	// evaluada en vivo (trigger LIVE o CATCH_UP), también NORMAL y
	// NO_CALCULABLE, después del commit. No se publica en el backfill, en
	// duplicados, en horas tardías ni con alerts_enabled = false (modo sombra).
	//
	// Clave de idempotencia del consumidor: (MeasuringPointID, EvaluatedAt).
	// La fuente de verdad es motion_evaluations (toda hora tiene fila).
	TopicMotionEvaluated = "rule_engine.motion_evaluated"

	// MotionSchemaVersion es la versión del esquema de MotionEvaluated.
	MotionSchemaVersion = 1

	// EventTypeMotion es el event_type de MotionEvaluated.
	EventTypeMotion = "MOVIMIENTO"
)

// Estados del movimiento (motion_state; vacío en una hora NO_CALCULABLE sin desplazamiento).
const (
	MotionStateNormal     = "NORMAL"
	MotionStateSalto      = "SALTO"
	MotionStateDesplazado = "DESPLAZADO"
	MotionStateRetorno    = "RETORNO"
	MotionStateBrusco     = "BRUSCO"
)

// Estados de la temperatura (temp_state; vacío con SOLO_MOVIMIENTO).
const (
	TempStateVariable       = "TEMP_VARIABLE"
	TempStateVerificando    = "VERIFICANDO"
	TempStateEstable        = "TEMP_ESTABLE"
	TempStateRetornoEstable = "RETORNO_TEMP_ESTABLE"
	TempStateSinEstabilizar = "SIN_ESTABILIZAR"
)

// Modos de temperatura.
const (
	TempModeMotionAndTemp = "MOVIMIENTO_Y_TEMPERATURA"
	TempModeMotionOnly    = "SOLO_MOVIMIENTO"
)

// Códigos de los motivos de movimiento y temperatura.
const (
	ReasonMovJump           = "MOV_JUMP"
	ReasonMovShift          = "MOV_SHIFT"
	ReasonMovShiftGap       = "MOV_SHIFT_GAP"
	ReasonMovReturn         = "MOV_RETURN"
	ReasonMovAbrupt         = "MOV_ABRUPT"
	ReasonMovStill          = "MOV_STILL"
	ReasonMovNewPosition    = "MOV_NEW_POSITION"
	ReasonTempVerifying     = "TEMP_VERIFYING"
	ReasonTempStabilized    = "TEMP_STABILIZED"
	ReasonTempNotStabilized = "TEMP_NOT_STABILIZED"
	ReasonTempVariesAgain   = "TEMP_VARIES_AGAIN"
	ReasonTempStillStable   = "TEMP_STILL_STABLE"
)

// MotionReason es un motivo de la hora. Data trae las claves del código:
//   - MOV_JUMP: dp, dr (null sin dp_1h), deriva.
//   - MOV_SHIFT / MOV_RETURN: dp, dr, started_at.
//   - MOV_SHIFT_GAP: dp, dr, gap_from, gap_to.
//   - MOV_ABRUPT: axis (pitch | roll), delta, hours.
//   - MOV_STILL / TEMP_NOT_STABILIZED: hours.
//   - TEMP_STABILIZED: temp_rate_max_6h, temp_stable_from.
//   - TEMP_VARIES_AGAIN: rate, temp_stable_from, temp_stable_to.
//   - TEMP_STILL_STABLE: hours, temp_rate_max_6h.
//   - MOV_NEW_POSITION / TEMP_VERIFYING: vacío.
//
// Los grados y °C/h van redondeados a 3 decimales; las fechas en RFC 3339 UTC.
type MotionReason struct {
	Code string         `json:"code"`
	Data map[string]any `json:"data"`
}

// MotionShift es el desplazamiento mientras displaced: Pitch y Roll = nivel
// actual − referencia (null si no está desplazado). GapFrom y GapTo solo si
// el desplazamiento se detectó por un hueco.
type MotionShift struct {
	Pitch     *float64   `json:"pitch"`
	Roll      *float64   `json:"roll"`
	StartedAt *time.Time `json:"started_at"`
	GapFrom   *time.Time `json:"gap_from"`
	GapTo     *time.Time `json:"gap_to"`
}

// TempStable es el tramo de temperatura estable (From en TEMP_ESTABLE,
// RETORNO_TEMP_ESTABLE o en la hora de TEMP_VARIES_AGAIN; To solo en esa hora).
type TempStable struct {
	From *time.Time `json:"from"`
	To   *time.Time `json:"to"`
}

// MotionEvaluated es la evaluación de movimiento de UNA hora del punto (la
// hora t cubre [t − 1 h, t)). Todos los time.Time están en UTC.
type MotionEvaluated struct {
	MotionEvaluationID int64     `json:"motion_evaluation_id"`
	MeasuringPointID   int64     `json:"measuring_point_id"`
	EventType          string    `json:"event_type"`
	EvaluatedAt        time.Time `json:"evaluated_at"`
	// Status es CALCULATED o NO_CALCULABLE.
	Status string `json:"status"`
	// MotionState vacío en una hora NO_CALCULABLE sin desplazamiento.
	MotionState string `json:"motion_state"`
	// TempState vacío con SOLO_MOVIMIENTO.
	TempState string `json:"temp_state"`
	TempMode  string `json:"temp_mode"`
	// ComputedLevel es AMARILLO dentro del episodio de movimiento, si no NORMAL.
	ComputedLevel   string     `json:"computed_level"`
	InEpisode       bool       `json:"in_episode"`
	EpisodeEvent    string     `json:"episode_event"`
	EpisodeOpenedAt *time.Time `json:"episode_opened_at"`

	Shift      MotionShift    `json:"shift"`
	TempStable TempStable     `json:"temp_stable"`
	Reasons    []MotionReason `json:"reasons"`

	ThresholdVersionID int64     `json:"threshold_version_id"`
	OccurredAt         time.Time `json:"occurred_at"`
	SchemaVersion      int       `json:"schema_version"`
}
