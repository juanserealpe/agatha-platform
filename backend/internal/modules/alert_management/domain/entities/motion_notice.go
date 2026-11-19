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
)

// MotionReason es un motivo de la hora de movimiento. Los campos que no
// aplican al código quedan en cero (ver rule_engine.motion_evaluated v1).
type MotionReason struct {
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
	// Del JSON los números llegan como float64; del bus en proceso, también como int.
	asFloat := func(v any) (float64, bool) {
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
		Code: code, DP: numPtr("dp"), DR: numPtr("dr"), Deriva: numPtr("deriva"),
		StartedAt: timePtr("started_at"), GapFrom: timePtr("gap_from"), GapTo: timePtr("gap_to"),
		Axis: axis, Delta: num("delta"), Hours: int(num("hours")),
		TempRateMax6h: numPtr("temp_rate_max_6h"), Rate: num("rate"),
		TempStableFrom: timePtr("temp_stable_from"), TempStableTo: timePtr("temp_stable_to"),
	}
}

// MotionNotice es la vista propia de Alert Management de una hora de
// movimiento (rule_engine.motion_evaluated v1, o una fila de
// motion_evaluations en el catch-up y el backfill). La hora t cubre [t − 1 h, t).
type MotionNotice struct {
	MotionEvaluationID int64
	MeasuringPointID   int64
	EvaluatedAt        time.Time
	// Status es CALCULATED o NO_CALCULABLE.
	Status        string
	MotionState   string // vacío en NO_CALCULABLE sin desplazamiento
	TempState     string // vacío con SOLO_MOVIMIENTO
	TempMode      string
	ComputedLevel Level
	InEpisode     bool
	EpisodeEvent  EpisodeEvent

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
