package entities

import "time"

// EventTypeMotion es el tipo de evento de las evaluaciones de movimiento.
const EventTypeMotion EventType = "MOVIMIENTO"

// MotionAlgorithmVersion identifica la versión del algoritmo MotionAssess.
const MotionAlgorithmVersion = "motion-v2"

// MotionStatus indica si la hora tuvo registros suficientes.
type MotionStatus string

const (
	MotionStatusCalculated   MotionStatus = "CALCULATED"
	MotionStatusNoCalculable MotionStatus = "NO_CALCULABLE"
)

// MotionState es el estado del movimiento de una hora (§4.1). Vacío ("") en
// una hora NO_CALCULABLE sin desplazamiento (NULL en la tabla).
type MotionStateValue string

const (
	MotionNormal     MotionStateValue = "NORMAL"
	MotionSalto      MotionStateValue = "SALTO"
	MotionDesplazado MotionStateValue = "DESPLAZADO"
	MotionRetorno    MotionStateValue = "RETORNO"
	MotionBrusco     MotionStateValue = "BRUSCO"
)

// TempState es el estado de la temperatura (§4.2). Vacío ("") con
// SOLO_MOVIMIENTO (NULL en la tabla).
type TempState string

const (
	TempVariable       TempState = "TEMP_VARIABLE"
	TempVerificando    TempState = "VERIFICANDO"
	TempEstable        TempState = "TEMP_ESTABLE"
	TempRetornoEstable TempState = "RETORNO_TEMP_ESTABLE"
	TempSinEstabilizar TempState = "SIN_ESTABILIZAR"
)

// TempMode decide si se ejecuta el paso de temperatura.
type TempMode string

const (
	TempModeMotionAndTemp TempMode = "MOVIMIENTO_Y_TEMPERATURA"
	TempModeMotionOnly    TempMode = "SOLO_MOVIMIENTO"
)

// MotionTrigger es quién originó la evaluación.
type MotionTrigger string

const (
	MotionTriggerLive     MotionTrigger = "LIVE"
	MotionTriggerCatchUp  MotionTrigger = "CATCH_UP"
	MotionTriggerBackfill MotionTrigger = "BACKFILL"
)

// Códigos de motivo de movimiento y temperatura (§6.2).
const (
	ReasonMovJump           ReasonCode = "MOV_JUMP"
	ReasonMovShift          ReasonCode = "MOV_SHIFT"
	ReasonMovShiftGap       ReasonCode = "MOV_SHIFT_GAP"
	ReasonMovReturn         ReasonCode = "MOV_RETURN"
	ReasonMovAbrupt         ReasonCode = "MOV_ABRUPT"
	ReasonMovStill          ReasonCode = "MOV_STILL"
	ReasonMovNewPosition    ReasonCode = "MOV_NEW_POSITION"
	ReasonMovCumulative     ReasonCode = "MOV_CUMULATIVE"
	ReasonMovLevelUp        ReasonCode = "MOV_LEVEL_UP"
	ReasonMovDaily          ReasonCode = "MOV_DAILY"
	ReasonTempVerifying     ReasonCode = "TEMP_VERIFYING"
	ReasonTempStabilized    ReasonCode = "TEMP_STABILIZED"
	ReasonTempNotStabilized ReasonCode = "TEMP_NOT_STABILIZED"
	ReasonTempVariesAgain   ReasonCode = "TEMP_VARIES_AGAIN"
	ReasonTempStillStable   ReasonCode = "TEMP_STILL_STABLE"
)

// RawRecord es un registro crudo agrupado por timestamp: métricas 1 (pitch),
// 2 (roll), 19 (deriva) y 20 (temperatura). nil = la métrica no vino.
type RawRecord struct {
	Timestamp time.Time
	Pitch     *float64
	Roll      *float64
	Deriva    *float64
	Temp      *float64
}

// MotionParams son los parámetros de una versión de motion_threshold_versions (§5.2).
type MotionParams struct {
	AlertsEnabled bool
	TempMode      TempMode

	RecordsPerHour int
	MinRecordsHour int

	JumpDeg       float64
	DerivaJumpDeg float64

	ShiftDeg         float64
	ShiftWindowHours int
	ShiftMinRecords  int

	BaseFromHours  int
	BaseToHours    int
	BaseMinRecords int

	GapMinHours       int
	GapPreHours       int
	GapPostMinutes    int
	GapPreMinRecords  int
	GapPostMinRecords int

	ReturnFraction                float64
	ShiftSuppressAfterReturnHours int

	AbruptDeg                     float64
	AbruptHours                   int
	AbruptSuppressAfterJointHours int
	SlowDriftDeg24h               float64

	StillHours         int
	StillMinCalculated int
	StillRangeDeg      float64

	TempFlatRate     float64
	TempFlatHours    int
	TempFlatMinRates int
	TempFlowRate     float64
	TempFlowHours    int

	StabilizeMaxHours int

	StillStableFirstHours int
	StillStableEveryHours int

	TrendDeg7d float64

	// motion-v2 (§5.2). Rules es la tabla de lo normal (motion_threshold_rules):
	// no es columna de la versión; AbruptDeg y AbruptHours ya no se usan.
	Rules []MotionRule

	SendNaranja MotionSendMode
	SendRojo    MotionSendMode

	LongHoldHours   int
	LongMinCoverage float64
	LongCloseHours  int
	LongCloseNetDeg float64

	ClosureCapLevel Level
	ClosureMaxDeg   float64

	DailySummary bool

	HistoryStepDeg         float64
	HistoryStepWindowHours int
	HistoryStepGroupHours  int
	HistoryStepShare       float64
	HistoryPreviousDays    int
}

// DefaultMotionParams son los valores v1 del MP-11556 (§5.2).
func DefaultMotionParams() MotionParams {
	return MotionParams{
		AlertsEnabled: true, TempMode: TempModeMotionAndTemp,
		RecordsPerHour: 4, MinRecordsHour: 3,
		JumpDeg: 0.03, DerivaJumpDeg: 0.03,
		ShiftDeg: 0.04, ShiftWindowHours: 2, ShiftMinRecords: 6,
		BaseFromHours: 24, BaseToHours: 6, BaseMinRecords: 36,
		GapMinHours: 1, GapPreHours: 2, GapPostMinutes: 45, GapPreMinRecords: 3, GapPostMinRecords: 2,
		ReturnFraction: 0.5, ShiftSuppressAfterReturnHours: 24,
		AbruptDeg: 0.04, AbruptHours: 3, AbruptSuppressAfterJointHours: 6, SlowDriftDeg24h: 0.04,
		StillHours: 24, StillMinCalculated: 20, StillRangeDeg: 0.02,
		TempFlatRate: 0.03, TempFlatHours: 6, TempFlatMinRates: 5,
		TempFlowRate: 0.08, TempFlowHours: 2,
		StabilizeMaxHours:     36,
		StillStableFirstHours: 6, StillStableEveryHours: 12,
		TrendDeg7d: 0.04,

		Rules:       DefaultMotionRules(),
		SendNaranja: MotionSendObservation, SendRojo: MotionSendObservation,
		LongHoldHours: 12, LongMinCoverage: 0.5, LongCloseHours: 72, LongCloseNetDeg: 0.02,
		ClosureCapLevel: LevelAmarillo, ClosureMaxDeg: 0.20,
		DailySummary:   true,
		HistoryStepDeg: 0.015, HistoryStepWindowHours: 6, HistoryStepGroupHours: 8, HistoryStepShare: 0.7,
		HistoryPreviousDays: 90,
	}
}

// NewPointMotionParams es la v1 de un punto nuevo: modo sombra y solo movimiento.
func NewPointMotionParams() MotionParams {
	p := DefaultMotionParams()
	p.AlertsEnabled = false
	p.TempMode = TempModeMotionOnly
	return p
}

// MotionThresholdVersion es una fila de motion_threshold_versions.
type MotionThresholdVersion struct {
	ID               int64
	MeasuringPointID int64
	Version          int
	// PreviousVersionID no es columna: lo completa el caso de uso.
	PreviousVersionID *int64
	// Warnings no es columna: los avisos de la tabla (§5.4) que devuelve CreateVersion.
	Warnings  []string
	Params    MotionParams
	ValidFrom time.Time
	ValidTo   *time.Time
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// PendingGap es un hueco cuya hora de evaluación fue NO_CALCULABLE.
type PendingGap struct {
	From time.Time // prev: último registro antes del hueco
	To   time.Time // f: primer registro después del hueco
}

// MotionState es el estado que pasa de una hora a la siguiente (§5.4). Se
// guarda en la fila y se lee de la anterior; el cero es "sin historia".
type MotionState struct {
	Displaced       bool
	RefPitch        *float64
	RefRoll         *float64
	ShiftSign       int
	ShiftStartedAt  *time.Time
	ShiftLevelPitch *float64
	ShiftLevelRoll  *float64
	GapFrom         *time.Time
	GapTo           *time.Time
	LastJointAt     *time.Time
	LastReturnAt    *time.Time
	ReturnStartedAt *time.Time
	TempState       TempState
	TempSince       *time.Time
	TempStableFrom  *time.Time
	TempStableTo    *time.Time
	StillStableMsgs int
	InEpisode       bool
	EpisodeOpenedAt *time.Time
	AbruptActive    bool
	PendingGaps     []PendingGap

	// motion-v2 (§6.1). Los Ep* solo valen dentro del episodio (fuera quedan
	// en cero, salvo EpKind en la hora del CLEARED). LongHold guarda solo las
	// claves con horas > 0.
	RestAt        *time.Time
	LongHold      map[string]int
	LastPatternAt *time.Time
	LastFastAt    *time.Time
	EpLevel       Level
	EpKind        MotionEpisodeKind
	EpStartAt     *time.Time
	EpRefPitch    *float64
	EpRefRoll     *float64
	EpHasLong     bool
}

// InitialMotionState es el estado antes de la primera hora de un punto.
func InitialMotionState(p MotionParams) MotionState {
	s := MotionState{}
	if p.TempMode == TempModeMotionAndTemp {
		s.TempState = TempVariable
	}
	return s
}

// MotionReason es un motivo de la hora. Los campos que no aplican al código
// quedan en cero; Data() arma el objeto {code, data} canónico.
//   - MOV_JUMP: DP, DR (nil si no hay dp_1h), Deriva (sin redondear).
//   - MOV_SHIFT / MOV_RETURN: DP, DR, StartedAt.
//   - MOV_SHIFT_GAP: DP, DR, GapFrom, GapTo.
//   - MOV_ABRUPT: Hit (umbral corto de mayor nivel), AmarilloDeg.
//   - MOV_CUMULATIVE: Hit (umbral largo de mayor nivel), AmarilloDeg, Steps,
//     StepList, Pattern, RatePerDay3d.
//   - MOV_LEVEL_UP: FromLevel, ToLevel, SentFrom, SentTo, Hit (nil si ningún
//     umbral tiene ese nivel).
//   - MOV_DAILY: NoData, CalculatedHours24h, Day; si hay datos, Level, Sent,
//     DP24h, DPTotal, DR24h, DRTotal.
//   - MOV_NEW_POSITION: DPTotal, DRTotal.
//   - MOV_STILL: Hours, DPTotal, DRTotal, StartedAt, Level (max_level), Sent
//     (max_sent), LongEpisode.
//   - TEMP_NOT_STABILIZED: Hours.
//   - TEMP_STABILIZED: TempRateMax6h, TempStableFrom.
//   - TEMP_VARIES_AGAIN: Rate, TempStableFrom, TempStableTo.
//   - TEMP_STILL_STABLE: Hours, TempRateMax6h (nil si no hay tasas).
//
// Los grados y °C/h van redondeados a 3 decimales, salvo Deriva; las
// velocidades por día, a 4.
type MotionReason struct {
	Code           ReasonCode
	DP             *float64
	DR             *float64
	Deriva         *float64
	StartedAt      *time.Time
	GapFrom        *time.Time
	GapTo          *time.Time
	Hours          int
	TempRateMax6h  *float64
	Rate           float64
	TempStableFrom *time.Time
	TempStableTo   *time.Time

	// motion-v2.
	Hit                *MotionRuleHit
	AmarilloDeg        *float64
	Steps              int
	StepList           []MotionStep
	Pattern            string
	RatePerDay3d       *float64
	FromLevel, ToLevel Level
	SentFrom, SentTo   Level
	Level, Sent        Level
	NoData             bool
	CalculatedHours24h int
	Day                int
	DP24h, DR24h       *float64
	DPTotal, DRTotal   *float64
	LongEpisode        bool
}

// Data devuelve el objeto data del motivo con las claves de cada código.
// Las fechas van en RFC 3339 UTC; nil queda como null.
func (r MotionReason) Data() map[string]any {
	switch r.Code {
	case ReasonMovJump:
		return map[string]any{"dp": fptr(r.DP), "dr": fptr(r.DR), "deriva": fptr(r.Deriva)}
	case ReasonMovShift, ReasonMovReturn:
		return map[string]any{"dp": fptr(r.DP), "dr": fptr(r.DR), "started_at": tptr(r.StartedAt)}
	case ReasonMovShiftGap:
		return map[string]any{"dp": fptr(r.DP), "dr": fptr(r.DR), "gap_from": tptr(r.GapFrom), "gap_to": tptr(r.GapTo)}
	case ReasonMovAbrupt:
		d := map[string]any{"amarillo_deg": fptr(r.AmarilloDeg)}
		if h := r.Hit; h != nil {
			d["axis"], d["delta"], d["hours"], d["window_hours"], d["level"], d["deg"] =
				h.Axis, h.Delta, h.SpanHours, h.WindowHours, string(h.Level), h.Deg
		}
		return d
	case ReasonMovCumulative:
		d := map[string]any{"amarillo_deg": fptr(r.AmarilloDeg), "steps": r.Steps, "pattern": r.Pattern,
			"rate_per_day_3d": fptr(r.RatePerDay3d)}
		steps := make([]any, len(r.StepList))
		for i, s := range r.StepList {
			steps[i] = map[string]any{"at": s.At.UTC().Format(time.RFC3339), "delta": s.Delta}
		}
		d["step_list"] = steps
		if h := r.Hit; h != nil {
			d["axis"], d["delta"], d["window_hours"], d["since"], d["span_hours"], d["level"], d["deg"] =
				h.Axis, h.Delta, h.WindowHours, tptr(h.Since), h.SpanHours, string(h.Level), h.Deg
		}
		return d
	case ReasonMovLevelUp:
		d := map[string]any{"from_level": string(r.FromLevel), "to_level": string(r.ToLevel),
			"sent_from": string(r.SentFrom), "sent_to": string(r.SentTo),
			"axis": nil, "delta": nil, "window_hours": nil, "span_hours": nil, "deg": nil}
		if h := r.Hit; h != nil {
			d["axis"], d["delta"], d["window_hours"], d["span_hours"], d["deg"] = h.Axis, h.Delta, h.WindowHours, h.SpanHours, h.Deg
		}
		return d
	case ReasonMovDaily:
		if r.NoData {
			return map[string]any{"no_data": true, "calculated_hours_24h": 0, "day": r.Day}
		}
		return map[string]any{"no_data": false, "calculated_hours_24h": r.CalculatedHours24h, "day": r.Day,
			"level": string(r.Level), "sent": string(r.Sent),
			"dp_24h": fptr(r.DP24h), "dp_total": fptr(r.DPTotal), "dr_24h": fptr(r.DR24h), "dr_total": fptr(r.DRTotal)}
	case ReasonMovNewPosition:
		return map[string]any{"dp_total": fptr(r.DPTotal), "dr_total": fptr(r.DRTotal)}
	case ReasonMovStill:
		return map[string]any{"hours": r.Hours, "dp_total": fptr(r.DPTotal), "dr_total": fptr(r.DRTotal),
			"started_at": tptr(r.StartedAt), "max_level": string(r.Level), "max_sent": string(r.Sent),
			"long_episode": r.LongEpisode}
	case ReasonTempNotStabilized:
		return map[string]any{"hours": r.Hours}
	case ReasonTempStabilized:
		return map[string]any{"temp_rate_max_6h": fptr(r.TempRateMax6h), "temp_stable_from": tptr(r.TempStableFrom)}
	case ReasonTempVariesAgain:
		return map[string]any{"rate": r.Rate, "temp_stable_from": tptr(r.TempStableFrom), "temp_stable_to": tptr(r.TempStableTo)}
	case ReasonTempStillStable:
		return map[string]any{"hours": r.Hours, "temp_rate_max_6h": fptr(r.TempRateMax6h)}
	default:
		return map[string]any{}
	}
}

func fptr(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func tptr(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.UTC().Format(time.RFC3339)
}

// MotionEvaluation es una fila de motion_evaluations: la hora t evaluada, que
// cubre [t − 1 h, t). Las horas NO_CALCULABLE también tienen fila.
type MotionEvaluation struct {
	ID               int64
	MeasuringPointID int64
	EvaluatedAt      time.Time
	Status           MotionStatus
	NRecords         int
	Coverage         float64

	// Variables (§5.3). nil = nula.
	PitchNow       *float64
	RollNow        *float64
	TempNow        *float64
	DP1h           *float64
	DR1h           *float64
	BasePitch      *float64
	BaseRoll       *float64
	DP6h           *float64
	DR6h           *float64
	ShiftRecords2h int
	ShiftSignNow   int
	DerivaMax1h    *float64
	Jump           bool
	Abrupt         bool
	DP24h          *float64
	DR24h          *float64
	SlowDriftFlag  bool
	TrendFlag      bool
	TempRate       *float64 // temp_rate(t)
	TempRateMax6h  *float64
	TempFlat       bool
	TempFlow       bool
	Still          bool

	// Hueco evaluado en esta hora (aunque no haya disparado).
	EvalGapFrom *time.Time
	EvalGapTo   *time.Time
	DPGap       *float64
	DRGap       *float64

	// State es el estado al final de la hora (§5.4).
	State MotionState

	// Resultado.
	MotionState  MotionStateValue
	EpisodeEvent EpisodeEvent
	// ComputedLevel es el nivel ENVIADO del episodio (NORMAL fuera).
	ComputedLevel Level
	Reasons       []MotionReason

	// motion-v2. TableLevelHour solo en horas CALCULATED (vacío en
	// NO_CALCULABLE); TableLevel es el nivel calculado del episodio (NORMAL
	// fuera); EpisodeKind dentro del episodio y en la hora del CLEARED;
	// RulesHit los umbrales que cuentan; History solo en la hora OPENED.
	TableLevelHour Level
	TableLevel     Level
	EpisodeKind    MotionEpisodeKind
	RulesHit       []MotionRuleHit
	History        *MotionHistory

	// Trazabilidad.
	ThresholdVersionID int64
	AlgorithmVersion   string
	Trigger            MotionTrigger
	InputMaxTS         *time.Time
	CreatedAt          time.Time
}

// ReferencePitch y ReferenceRoll son la referencia solo mientras está desplazado.
func (e MotionEvaluation) ReferencePitch() *float64 {
	if !e.State.Displaced {
		return nil
	}
	return e.State.RefPitch
}

func (e MotionEvaluation) ReferenceRoll() *float64 {
	if !e.State.Displaced {
		return nil
	}
	return e.State.RefRoll
}

// ShiftStartedAt es el inicio del desplazamiento, solo mientras está desplazado.
func (e MotionEvaluation) ShiftStartedAt() *time.Time {
	if !e.State.Displaced {
		return nil
	}
	return e.State.ShiftStartedAt
}

// HasReason dice si la hora tiene un motivo con ese código.
func (e MotionEvaluation) HasReason(code ReasonCode) bool {
	for _, r := range e.Reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

// TempStableFrom es el inicio del tramo de temperatura estable: se informa en
// TEMP_ESTABLE, RETORNO_TEMP_ESTABLE o en la hora de TEMP_VARIES_AGAIN.
func (e MotionEvaluation) TempStableFrom() *time.Time {
	s := e.State.TempState
	if s == TempEstable || s == TempRetornoEstable || e.HasReason(ReasonTempVariesAgain) {
		return e.State.TempStableFrom
	}
	return nil
}

// TempStableTo es el fin del tramo, solo en la hora de TEMP_VARIES_AGAIN.
func (e MotionEvaluation) TempStableTo() *time.Time {
	if e.HasReason(ReasonTempVariesAgain) {
		return e.State.TempStableTo
	}
	return nil
}
