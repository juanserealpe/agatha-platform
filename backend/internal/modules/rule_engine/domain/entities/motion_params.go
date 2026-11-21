package entities

import (
	"sort"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
)

// MotionParamKind es el tipo de un parámetro de movimiento.
type MotionParamKind int

const (
	MotionParamFloat MotionParamKind = iota // grados o °C/h: NUMERIC(6,4)
	MotionParamInt                          // horas, minutos o registros
	MotionParamBool
	MotionParamTempMode
	MotionParamFraction // (0, 1]: NUMERIC(3,2)
	MotionParamSendMode // ACTIVE | OBSERVATION
	MotionParamLevel    // AMARILLO | NARANJA | ROJO
)

// MotionRulesParam es la clave del parche que trae la tabla de lo normal. No
// es columna: reemplaza la tabla completa (motion_threshold_rules).
const MotionRulesParam = "rules"

// MotionParamField describe un parámetro de motion_threshold_versions: su
// nombre en la API y la tabla, su tipo y el campo de MotionParams. Retired =
// columna que se conserva por las filas motion-v1 pero que el parche y la API
// ya no aceptan ni muestran (abrupt_deg, abrupt_hours).
type MotionParamField struct {
	Name    string
	Kind    MotionParamKind
	Retired bool
	ptr     func(p *MotionParams) any
}

// motionParamFields es la tabla única de parámetros (§5.2 de v1 y v2): la
// usan el parche, la API, el seed y la validación. El orden es el de la
// especificación.
var motionParamFields = []MotionParamField{
	{"alerts_enabled", MotionParamBool, false, func(p *MotionParams) any { return &p.AlertsEnabled }},
	{"temp_mode", MotionParamTempMode, false, func(p *MotionParams) any { return &p.TempMode }},
	{"records_per_hour", MotionParamInt, false, func(p *MotionParams) any { return &p.RecordsPerHour }},
	{"min_records_hour", MotionParamInt, false, func(p *MotionParams) any { return &p.MinRecordsHour }},
	{"jump_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.JumpDeg }},
	{"deriva_jump_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.DerivaJumpDeg }},
	{"shift_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.ShiftDeg }},
	{"shift_window_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.ShiftWindowHours }},
	{"shift_min_records", MotionParamInt, false, func(p *MotionParams) any { return &p.ShiftMinRecords }},
	{"base_from_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.BaseFromHours }},
	{"base_to_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.BaseToHours }},
	{"base_min_records", MotionParamInt, false, func(p *MotionParams) any { return &p.BaseMinRecords }},
	{"gap_min_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.GapMinHours }},
	{"gap_pre_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.GapPreHours }},
	{"gap_post_minutes", MotionParamInt, false, func(p *MotionParams) any { return &p.GapPostMinutes }},
	{"gap_pre_min_records", MotionParamInt, false, func(p *MotionParams) any { return &p.GapPreMinRecords }},
	{"gap_post_min_records", MotionParamInt, false, func(p *MotionParams) any { return &p.GapPostMinRecords }},
	{"return_fraction", MotionParamFloat, false, func(p *MotionParams) any { return &p.ReturnFraction }},
	{"shift_suppress_after_return_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.ShiftSuppressAfterReturnHours }},
	{"abrupt_deg", MotionParamFloat, true, func(p *MotionParams) any { return &p.AbruptDeg }},
	{"abrupt_hours", MotionParamInt, true, func(p *MotionParams) any { return &p.AbruptHours }},
	{"abrupt_suppress_after_joint_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.AbruptSuppressAfterJointHours }},
	{"slow_drift_deg_24h", MotionParamFloat, false, func(p *MotionParams) any { return &p.SlowDriftDeg24h }},
	{"still_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.StillHours }},
	{"still_min_calculated", MotionParamInt, false, func(p *MotionParams) any { return &p.StillMinCalculated }},
	{"still_range_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.StillRangeDeg }},
	{"temp_flat_rate", MotionParamFloat, false, func(p *MotionParams) any { return &p.TempFlatRate }},
	{"temp_flat_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.TempFlatHours }},
	{"temp_flat_min_rates", MotionParamInt, false, func(p *MotionParams) any { return &p.TempFlatMinRates }},
	{"temp_flow_rate", MotionParamFloat, false, func(p *MotionParams) any { return &p.TempFlowRate }},
	{"temp_flow_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.TempFlowHours }},
	{"stabilize_max_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.StabilizeMaxHours }},
	{"still_stable_first_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.StillStableFirstHours }},
	{"still_stable_every_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.StillStableEveryHours }},
	{"trend_deg_7d", MotionParamFloat, false, func(p *MotionParams) any { return &p.TrendDeg7d }},
	// motion-v2 (§5.2).
	{"send_naranja", MotionParamSendMode, false, func(p *MotionParams) any { return &p.SendNaranja }},
	{"send_rojo", MotionParamSendMode, false, func(p *MotionParams) any { return &p.SendRojo }},
	{"long_hold_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.LongHoldHours }},
	{"long_min_coverage", MotionParamFraction, false, func(p *MotionParams) any { return &p.LongMinCoverage }},
	{"long_close_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.LongCloseHours }},
	{"long_close_net_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.LongCloseNetDeg }},
	{"closure_cap_level", MotionParamLevel, false, func(p *MotionParams) any { return &p.ClosureCapLevel }},
	{"closure_max_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.ClosureMaxDeg }},
	{"daily_summary", MotionParamBool, false, func(p *MotionParams) any { return &p.DailySummary }},
	{"history_step_deg", MotionParamFloat, false, func(p *MotionParams) any { return &p.HistoryStepDeg }},
	{"history_step_window_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.HistoryStepWindowHours }},
	{"history_step_group_hours", MotionParamInt, false, func(p *MotionParams) any { return &p.HistoryStepGroupHours }},
	{"history_step_share", MotionParamFraction, false, func(p *MotionParams) any { return &p.HistoryStepShare }},
	{"history_previous_days", MotionParamInt, false, func(p *MotionParams) any { return &p.HistoryPreviousDays }},
}

// MotionParamFields devuelve la tabla de parámetros (copia), retirados incluidos.
func MotionParamFields() []MotionParamField {
	return append([]MotionParamField(nil), motionParamFields...)
}

// LookupMotionParam busca un parámetro por nombre (también los retirados).
func LookupMotionParam(name string) (MotionParamField, bool) {
	for _, f := range motionParamFields {
		if f.Name == name {
			return f, true
		}
	}
	return MotionParamField{}, false
}

// Get devuelve el valor del parámetro en p (float64, int, bool, TempMode,
// MotionSendMode o Level).
func (f MotionParamField) Get(p MotionParams) any {
	switch v := f.ptr(&p).(type) {
	case *float64:
		return *v
	case *int:
		return *v
	case *bool:
		return *v
	case *TempMode:
		return *v
	case *MotionSendMode:
		return *v
	case *Level:
		return *v
	}
	return nil
}

// GetString es Get con los enums como string (para la API y el seed).
func (f MotionParamField) GetString(p MotionParams) any {
	switch v := f.Get(p).(type) {
	case TempMode:
		return string(v)
	case MotionSendMode:
		return string(v)
	case Level:
		return string(v)
	default:
		return v
	}
}

// set asigna v si tiene el tipo del parámetro.
func (f MotionParamField) set(p *MotionParams, v any) bool {
	switch dst := f.ptr(p).(type) {
	case *float64:
		x, ok := v.(float64)
		*dst = x
		return ok
	case *int:
		x, ok := v.(int)
		*dst = x
		return ok
	case *bool:
		x, ok := v.(bool)
		*dst = x
		return ok
	case *TempMode:
		switch x := v.(type) {
		case TempMode:
			*dst = x
		case string:
			*dst = TempMode(x)
		default:
			return false
		}
		return true
	case *MotionSendMode:
		switch x := v.(type) {
		case MotionSendMode:
			*dst = x
		case string:
			*dst = MotionSendMode(x)
		default:
			return false
		}
		return true
	case *Level:
		switch x := v.(type) {
		case Level:
			*dst = x
		case string:
			*dst = Level(x)
		default:
			return false
		}
		return true
	}
	return false
}

// MotionPatch son los parámetros informados de una versión nueva, por nombre
// (los demás se copian de la vigente). Los valores son float64, int, bool o
// el enum del parámetro; "rules" es []MotionRule y reemplaza la tabla entera.
type MotionPatch map[string]any

// ApplyTo copia de base lo que el parche no informa. Un nombre desconocido o
// un valor de otro tipo es ErrInvalidPatch; abrupt_deg / abrupt_hours,
// ErrMotionAbruptRetired. Sin "rules" se copia la tabla de base.
func (patch MotionPatch) ApplyTo(base MotionParams) (MotionParams, error) {
	p := base
	p.Rules = CloneMotionRules(base.Rules)
	for name, v := range patch {
		if name == MotionRulesParam {
			rules, ok := v.([]MotionRule)
			if !ok {
				return MotionParams{}, domain.ErrInvalidPatch
			}
			p.Rules = CloneMotionRules(rules)
			continue
		}
		f, ok := LookupMotionParam(name)
		if !ok {
			return MotionParams{}, domain.ErrInvalidPatch
		}
		if f.Retired {
			return MotionParams{}, domain.ErrMotionAbruptRetired
		}
		if !f.set(&p, v) {
			return MotionParams{}, domain.ErrInvalidPatch
		}
	}
	return p, nil
}

// Normalize redondea grados y °C/h a la precisión de la columna (4
// decimales; fracciones a 2), normaliza los enums y ordena la tabla por
// ventana y eje. Se valida DESPUÉS de normalizar.
func (p MotionParams) Normalize() MotionParams {
	for _, f := range motionParamFields {
		switch f.Kind {
		case MotionParamFloat:
			v := f.ptr(&p).(*float64)
			*v = round4(*v)
		case MotionParamFraction:
			v := f.ptr(&p).(*float64)
			*v = round2(*v)
		}
	}
	norm := func(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }
	p.TempMode = TempMode(norm(string(p.TempMode)))
	p.SendNaranja = MotionSendMode(norm(string(p.SendNaranja)))
	p.SendRojo = MotionSendMode(norm(string(p.SendRojo)))
	p.ClosureCapLevel = Level(norm(string(p.ClosureCapLevel)))
	rules := CloneMotionRules(p.Rules)
	for i := range rules {
		rules[i].Axis = norm(rules[i].Axis)
		rules[i].AmarilloDeg = round4(rules[i].AmarilloDeg)
		if rules[i].NaranjaDeg != nil {
			*rules[i].NaranjaDeg = round4(*rules[i].NaranjaDeg)
		}
		if rules[i].RojoDeg != nil {
			*rules[i].RojoDeg = round4(*rules[i].RojoDeg)
		}
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].WindowHours != rules[j].WindowHours {
			return rules[i].WindowHours < rules[j].WindowHours
		}
		return rules[i].Axis < rules[j].Axis
	})
	p.Rules = rules
	return p
}

// Validate aplica las reglas de los CHECK de motion_threshold_versions y de
// la tabla de lo normal (§5.4).
func (p MotionParams) Validate() error {
	if p.TempMode != TempModeMotionAndTemp && p.TempMode != TempModeMotionOnly {
		return domain.ErrMotionTempMode
	}
	if !(p.JumpDeg > 0 && p.JumpDeg <= p.ShiftDeg) {
		return domain.ErrMotionJumpShift
	}
	if !(p.ReturnFraction > 0 && p.ReturnFraction <= 1) {
		return domain.ErrMotionReturnFraction
	}
	for _, s := range []MotionSendMode{p.SendNaranja, p.SendRojo} {
		if s != MotionSendActive && s != MotionSendObservation {
			return domain.ErrMotionSendMode
		}
	}
	if p.ClosureCapLevel != LevelAmarillo && p.ClosureCapLevel != LevelNaranja && p.ClosureCapLevel != LevelRojo {
		return domain.ErrMotionClosureCap
	}
	for _, f := range motionParamFields {
		switch f.Kind {
		case MotionParamInt:
			if *f.ptr(&p).(*int) < 1 {
				return domain.ErrMotionHoursRange
			}
		case MotionParamFloat:
			if *f.ptr(&p).(*float64) <= 0 {
				return domain.ErrMotionDegreesRange
			}
		case MotionParamFraction:
			if v := *f.ptr(&p).(*float64); !(v > 0 && v <= 1) {
				return domain.ErrMotionFraction
			}
		}
	}
	if p.MinRecordsHour > p.RecordsPerHour {
		return domain.ErrMotionRecordsRange
	}
	return ValidateMotionRules(p.Rules)
}

// ValidateMotionRules valida la tabla de lo normal (§5.4, 400).
func ValidateMotionRules(rules []MotionRule) error {
	if len(rules) == 0 {
		return domain.ErrMotionRulesEmpty
	}
	axes := map[int]map[string]bool{}
	for _, r := range rules {
		if r.WindowHours < 1 || r.WindowHours > MotionRuleMaxHours {
			return domain.ErrMotionRuleWindow
		}
		if r.Axis != MotionAxisAny && r.Axis != MotionAxisPitch && r.Axis != MotionAxisRoll {
			return domain.ErrMotionRuleAxis
		}
		if r.AmarilloDeg <= MotionMinAmarilloDeg+1e-9 {
			return domain.ErrMotionRuleAmarillo
		}
		prev := r.AmarilloDeg
		if r.NaranjaDeg != nil {
			if *r.NaranjaDeg <= prev {
				return domain.ErrMotionRuleOrder
			}
			prev = *r.NaranjaDeg
		}
		if r.RojoDeg != nil && *r.RojoDeg <= prev {
			return domain.ErrMotionRuleOrder
		}
		if axes[r.WindowHours] == nil {
			axes[r.WindowHours] = map[string]bool{}
		}
		seen := axes[r.WindowHours]
		if seen[r.Axis] || (r.Axis == MotionAxisAny && len(seen) > 0) || (r.Axis != MotionAxisAny && seen[MotionAxisAny]) {
			return domain.ErrMotionRuleDuplicate
		}
		seen[r.Axis] = true
	}
	return nil
}

// Avisos de la tabla (§5.4): se guarda igual y se devuelven en warnings[].
const (
	MotionWarnNormalNotIncreasing     = "NORMAL_NOT_INCREASING"
	MotionWarnNoLongRule              = "NO_LONG_RULE"
	MotionWarnNoShortRule             = "NO_SHORT_RULE"
	MotionWarnWindowLongerThanHistory = "WINDOW_LONGER_THAN_HISTORY"
)

// MotionRuleWarnings devuelve los avisos de la tabla. historyHours es el
// largo del historial del punto en horas (< 0 = desconocido: no se avisa).
func MotionRuleWarnings(rules []MotionRule, historyHours float64) []string {
	var out []string
	add := func(w string) {
		for _, x := range out {
			if x == w {
				return
			}
		}
		out = append(out, w)
	}
	hasShort, hasLong := false, false
	for _, a := range rules {
		if a.Kind() == MotionRuleShort {
			hasShort = true
		} else {
			hasLong = true
		}
		if historyHours >= 0 && float64(a.WindowHours) > historyHours {
			add(MotionWarnWindowLongerThanHistory)
		}
		for _, b := range rules {
			sameAxis := a.Axis == b.Axis || a.Axis == MotionAxisAny || b.Axis == MotionAxisAny
			if sameAxis && b.WindowHours > a.WindowHours && b.AmarilloDeg < a.AmarilloDeg-1e-9 {
				add(MotionWarnNormalNotIncreasing)
			}
		}
	}
	if !hasLong {
		add(MotionWarnNoLongRule)
	}
	if !hasShort {
		add(MotionWarnNoShortRule)
	}
	return out
}

// Window devuelve la ventana de registros crudos que necesita la hora t.
func MotionWindow(t time.Time) (time.Time, time.Time) {
	return t.Add(-MotionWindowHours * time.Hour), t
}

// MotionWindowHours es el largo de la ventana que necesita MotionAssess
// motion-v2: el historial de 30 días mide hasta tb − 30 d − 6 h (727 h antes
// de tb) y tb puede quedar hasta ~169 h antes de t (el inicio de un hueco),
// y la regla de 720 h mira hasta t − 723 h. 900 h cubre los dos.
const MotionWindowHours = 900

// MotionPilotPointID es el punto de prueba (MP-11556): el seed le da la v1 de
// §5.2; al resto, la de punto nuevo (modo sombra, SOLO_MOVIMIENTO).
const MotionPilotPointID int64 = 11556

// IsEffectiveAt indica si la versión aplica a la hora t.
func (v MotionThresholdVersion) IsEffectiveAt(t time.Time) bool {
	return !v.ValidFrom.After(t) && (v.ValidTo == nil || v.ValidTo.After(t))
}
