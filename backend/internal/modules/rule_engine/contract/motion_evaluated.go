package contract

import (
	"encoding/json"
	"time"
)

const (
	// TopicMotionEvaluated se publica por cada punto y hora de movimiento
	// evaluada en vivo (trigger LIVE o CATCH_UP), también NORMAL y
	// NO_CALCULABLE, después del commit. No se publica en el backfill, en
	// duplicados, en horas tardías ni con alerts_enabled = false (modo sombra).
	//
	// Clave de idempotencia del consumidor: (MeasuringPointID, EvaluatedAt).
	// La fuente de verdad es motion_evaluations (toda hora tiene fila).
	TopicMotionEvaluated = "rule_engine.motion_evaluated"

	// MotionSchemaVersion es la versión del esquema de MotionEvaluated. La v2
	// (motion-v2) es aditiva: agrega table_level, episode_kind, rules_hit y
	// history. Un consumidor de v1 la puede leer igual.
	MotionSchemaVersion = 2

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
	ReasonMovCumulative     = "MOV_CUMULATIVE"
	ReasonMovLevelUp        = "MOV_LEVEL_UP"
	ReasonMovDaily          = "MOV_DAILY"
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
//   - MOV_ABRUPT: axis (pitch | roll), delta, hours, window_hours, level,
//     deg, amarillo_deg.
//   - MOV_CUMULATIVE: axis, delta, window_hours, since, span_hours, level,
//     deg, amarillo_deg, steps, step_list [{at, delta}], pattern
//     (ESCALONES | UN_ESCALON | CONTINUO), rate_per_day_3d.
//   - MOV_LEVEL_UP: from_level, to_level, sent_from, sent_to, axis, delta,
//     window_hours, span_hours, deg.
//   - MOV_DAILY: no_data, calculated_hours_24h, day; con datos, además
//     level, sent, dp_24h, dp_total, dr_24h, dr_total (null si falta una mediana).
//   - MOV_NEW_POSITION: dp_total, dr_total.
//   - MOV_STILL: hours, dp_total, dr_total, started_at, max_level, max_sent,
//     long_episode.
//   - TEMP_NOT_STABILIZED: hours.
//   - TEMP_STABILIZED: temp_rate_max_6h, temp_stable_from.
//   - TEMP_VARIES_AGAIN: rate, temp_stable_from, temp_stable_to.
//   - TEMP_STILL_STABLE: hours, temp_rate_max_6h.
//   - TEMP_VERIFYING: vacío.
//
// Los grados y °C/h van redondeados a 3 decimales (las velocidades por día a
// 4); las fechas en RFC 3339 UTC. Los números llegan como float64.
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
	// ComputedLevel es el nivel ENVIADO dentro del episodio (el más alto
	// activo que no supera TableLevel), si no NORMAL.
	ComputedLevel string `json:"computed_level"`
	// TableLevel es el nivel calculado del episodio (no baja), si no NORMAL.
	TableLevel string `json:"table_level"`
	// EpisodeKind (SALTO | HUECO | RAPIDO | ACUMULADO) dentro del episodio y
	// en la hora del CLEARED; vacío fuera.
	EpisodeKind string `json:"episode_kind"`
	// RulesHit son los umbrales que cuentan en la hora: [{window_hours, kind,
	// level, deg, axis, delta, span_hours, since?}]. Vacío en NO_CALCULABLE.
	RulesHit []any `json:"rules_hit"`
	// History es el historial (§6.6) solo en la hora OPENED; si no, null.
	History map[string]any `json:"history"`

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

// JSONObject devuelve v como lo vería un consumidor que lee el JSON del
// evento (mapas, slices, float64, string, bool, nil). Así el bus en proceso
// entrega lo mismo que uno serializado.
func JSONObject(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// JSONArray es JSONObject para un arreglo ([] si v es nil o vacío).
func JSONArray(v any) []any {
	out := []any{}
	if b, err := json.Marshal(v); err == nil {
		var x []any
		if json.Unmarshal(b, &x) == nil && x != nil {
			out = x
		}
	}
	return out
}
