package entities

import "time"

// Estados del movimiento y de la temperatura (los mismos valores que el Rule Engine).
const (
	MotionDisplaced = "DESPLAZADO"

	TempEstable        = "TEMP_ESTABLE"
	TempRetornoEstable = "RETORNO_TEMP_ESTABLE"
)

// Códigos de motivo de movimiento y temperatura que interpreta Alert Management.
const (
	MotionJump              = "MOV_JUMP"
	MotionShift             = "MOV_SHIFT"
	MotionShiftGap          = "MOV_SHIFT_GAP"
	MotionReturn            = "MOV_RETURN"
	MotionAbrupt            = "MOV_ABRUPT"
	MotionStill             = "MOV_STILL"
	MotionNewPosition       = "MOV_NEW_POSITION"
	MotionTempStabilized    = "TEMP_STABILIZED"
	MotionTempNotStabilized = "TEMP_NOT_STABILIZED"
	MotionTempVariesAgain   = "TEMP_VARIES_AGAIN"
	MotionTempStillStable   = "TEMP_STILL_STABLE"
	// motion-v2.
	MotionCumulative = "MOV_CUMULATIVE"
	MotionLevelUp    = "MOV_LEVEL_UP"
	MotionDaily      = "MOV_DAILY"
)

// Tipos de episodio de movimiento (motion-v2, episode_kind).
const (
	MotionKindSalto     = "SALTO"
	MotionKindHueco     = "HUECO"
	MotionKindRapido    = "RAPIDO"
	MotionKindAcumulado = "ACUMULADO"
)

// MotionReason es un motivo de la hora de movimiento. Los campos que no
// aplican al código quedan en cero (ver rule_engine.motion_evaluated v1).
// Data es el objeto data tal como llegó: los motivos de motion-v2 (y los
// campos nuevos de MOV_ABRUPT, MOV_STILL...) se leen con Num, Int, Str, Bool,
// Time y Map.
type MotionReason struct {
	Data           map[string]any
	Code           string
	DP, DR         *float64
	Deriva         *float64
	StartedAt      *time.Time
	GapFrom, GapTo *time.Time
	Axis           string
	Delta          float64
	Hours          int
	TempRateMax6h  *float64
	Rate           float64
	TempStableFrom *time.Time
	TempStableTo   *time.Time
}

// ParseMotionReason arma el motivo desde su objeto data (números como
// float64 y fechas RFC 3339, como llegan del JSON). La usan la traducción del
// evento (bootstrap) y la lectura de motion_evaluations.
func ParseMotionReason(code string, data map[string]any) MotionReason {
	num := func(k string) float64 { f, _ := asFloat(data[k]); return f }
	numPtr := func(k string) *float64 {
		f, ok := asFloat(data[k])
		if !ok {
			return nil
		}
		return &f
	}
	timePtr := func(k string) *time.Time {
		s, ok := data[k].(string)
		if !ok {
			return nil
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil
		}
		t = t.UTC()
		return &t
	}
	axis, _ := data["axis"].(string)
	return MotionReason{
		Data: data,
		Code: code, DP: numPtr("dp"), DR: numPtr("dr"), Deriva: numPtr("deriva"),
		StartedAt: timePtr("started_at"), GapFrom: timePtr("gap_from"), GapTo: timePtr("gap_to"),
		Axis: axis, Delta: num("delta"), Hours: int(num("hours")),
		TempRateMax6h: numPtr("temp_rate_max_6h"), Rate: num("rate"),
		TempStableFrom: timePtr("temp_stable_from"), TempStableTo: timePtr("temp_stable_to"),
	}
}

// asFloat lee un número de data (float64 del JSON; int del bus en proceso).
func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

// value es data[k]; si data no la trae (un motivo armado en código o de un
// evento v1), cae a los campos tipados de v1 (dp, dr, axis, delta, hours).
func (r MotionReason) value(k string) any {
	if v, ok := r.Data[k]; ok {
		return v
	}
	switch k {
	case "dp":
		if r.DP != nil {
			return *r.DP
		}
	case "dr":
		if r.DR != nil {
			return *r.DR
		}
	case "axis":
		if r.Axis != "" {
			return r.Axis
		}
	case "delta":
		if r.Axis != "" {
			return r.Delta
		}
	case "hours":
		if r.Hours != 0 {
			return r.Hours
		}
	}
	return nil
}

// Num devuelve data[k] como número (nil si falta o es null).
func (r MotionReason) Num(k string) *float64 {
	f, ok := asFloat(r.value(k))
	if !ok {
		return nil
	}
	return &f
}

// Int devuelve data[k] como entero (0 si falta).
func (r MotionReason) Int(k string) int {
	f, _ := asFloat(r.value(k))
	return int(f)
}

// Str devuelve data[k] como texto ("" si falta).
func (r MotionReason) Str(k string) string {
	s, _ := r.value(k).(string)
	return s
}

// Bool devuelve data[k] como booleano (false si falta).
func (r MotionReason) Bool(k string) bool {
	b, _ := r.Data[k].(bool)
	return b
}

// Time devuelve data[k] (RFC 3339) como hora UTC (nil si falta).
func (r MotionReason) Time(k string) *time.Time {
	s, ok := r.Data[k].(string)
	if !ok {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

// MotionNotice es la vista propia de Alert Management de una hora de
// movimiento (rule_engine.motion_evaluated v1, o una fila de
// motion_evaluations en el catch-up y el backfill). La hora t cubre [t − 1 h, t).
type MotionNotice struct {
	MotionEvaluationID int64
	MeasuringPointID   int64
	EvaluatedAt        time.Time
	// Status es CALCULATED o NO_CALCULABLE.
	Status      string
	MotionState string // vacío en NO_CALCULABLE sin desplazamiento
	TempState   string // vacío con SOLO_MOVIMIENTO
	TempMode    string
	// ComputedLevel es el nivel ENVIADO del episodio (NORMAL fuera).
	ComputedLevel Level
	InEpisode     bool
	EpisodeEvent  EpisodeEvent

	// motion-v2 (evento v2). TableLevel es el nivel calculado del episodio;
	// EpisodeKind, SALTO | HUECO | RAPIDO | ACUMULADO; RulesHit, los umbrales
	// de la hora; History, el historial (solo en OPENED). Vacíos con v1.
	TableLevel    Level
	EpisodeKind   string
	RulesHit      []map[string]any
	History       map[string]any
	SchemaVersion int

	// Desplazamiento mientras displaced (nil si no).
	ShiftPitch, ShiftRoll *float64
	ShiftStartedAt        *time.Time
	GapFrom, GapTo        *time.Time

	TempStableFrom *time.Time
	TempStableTo   *time.Time
	Reasons        []MotionReason

	ThresholdVersionID int64
	// AlertsEnabled = false es modo sombra: la hora no abre ni toca alertas.
	AlertsEnabled bool
}

// Calculated indica si la hora tuvo registros suficientes.
func (n MotionNotice) Calculated() bool { return n.Status == StatusCalculated }

// Reason devuelve el motivo con ese código (nil si no está).
func (n MotionNotice) Reason(code string) *MotionReason {
	for i := range n.Reasons {
		if n.Reasons[i].Code == code {
			return &n.Reasons[i]
		}
	}
	return nil
}
