package entities

import (
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
)

// MotionParamField describe un parámetro de motion_threshold_versions: su
// nombre en la API y la tabla, su tipo y el campo de MotionParams.
type MotionParamField struct {
	Name string
	Kind MotionParamKind
	ptr  func(p *MotionParams) any
}

// motionParamFields es la tabla única de parámetros (§5.2): la usan el parche,
// la API y la validación. El orden es el de la especificación.
var motionParamFields = []MotionParamField{
	{"alerts_enabled", MotionParamBool, func(p *MotionParams) any { return &p.AlertsEnabled }},
	{"temp_mode", MotionParamTempMode, func(p *MotionParams) any { return &p.TempMode }},
	{"records_per_hour", MotionParamInt, func(p *MotionParams) any { return &p.RecordsPerHour }},
	{"min_records_hour", MotionParamInt, func(p *MotionParams) any { return &p.MinRecordsHour }},
	{"jump_deg", MotionParamFloat, func(p *MotionParams) any { return &p.JumpDeg }},
	{"deriva_jump_deg", MotionParamFloat, func(p *MotionParams) any { return &p.DerivaJumpDeg }},
	{"shift_deg", MotionParamFloat, func(p *MotionParams) any { return &p.ShiftDeg }},
	{"shift_window_hours", MotionParamInt, func(p *MotionParams) any { return &p.ShiftWindowHours }},
	{"shift_min_records", MotionParamInt, func(p *MotionParams) any { return &p.ShiftMinRecords }},
	{"base_from_hours", MotionParamInt, func(p *MotionParams) any { return &p.BaseFromHours }},
	{"base_to_hours", MotionParamInt, func(p *MotionParams) any { return &p.BaseToHours }},
	{"base_min_records", MotionParamInt, func(p *MotionParams) any { return &p.BaseMinRecords }},
	{"gap_min_hours", MotionParamInt, func(p *MotionParams) any { return &p.GapMinHours }},
	{"gap_pre_hours", MotionParamInt, func(p *MotionParams) any { return &p.GapPreHours }},
	{"gap_post_minutes", MotionParamInt, func(p *MotionParams) any { return &p.GapPostMinutes }},
	{"gap_pre_min_records", MotionParamInt, func(p *MotionParams) any { return &p.GapPreMinRecords }},
	{"gap_post_min_records", MotionParamInt, func(p *MotionParams) any { return &p.GapPostMinRecords }},
	{"return_fraction", MotionParamFloat, func(p *MotionParams) any { return &p.ReturnFraction }},
	{"shift_suppress_after_return_hours", MotionParamInt, func(p *MotionParams) any { return &p.ShiftSuppressAfterReturnHours }},
	{"abrupt_deg", MotionParamFloat, func(p *MotionParams) any { return &p.AbruptDeg }},
	{"abrupt_hours", MotionParamInt, func(p *MotionParams) any { return &p.AbruptHours }},
	{"abrupt_suppress_after_joint_hours", MotionParamInt, func(p *MotionParams) any { return &p.AbruptSuppressAfterJointHours }},
	{"slow_drift_deg_24h", MotionParamFloat, func(p *MotionParams) any { return &p.SlowDriftDeg24h }},
	{"still_hours", MotionParamInt, func(p *MotionParams) any { return &p.StillHours }},
	{"still_min_calculated", MotionParamInt, func(p *MotionParams) any { return &p.StillMinCalculated }},
	{"still_range_deg", MotionParamFloat, func(p *MotionParams) any { return &p.StillRangeDeg }},
	{"temp_flat_rate", MotionParamFloat, func(p *MotionParams) any { return &p.TempFlatRate }},
	{"temp_flat_hours", MotionParamInt, func(p *MotionParams) any { return &p.TempFlatHours }},
	{"temp_flat_min_rates", MotionParamInt, func(p *MotionParams) any { return &p.TempFlatMinRates }},
	{"temp_flow_rate", MotionParamFloat, func(p *MotionParams) any { return &p.TempFlowRate }},
	{"temp_flow_hours", MotionParamInt, func(p *MotionParams) any { return &p.TempFlowHours }},
	{"stabilize_max_hours", MotionParamInt, func(p *MotionParams) any { return &p.StabilizeMaxHours }},
	{"still_stable_first_hours", MotionParamInt, func(p *MotionParams) any { return &p.StillStableFirstHours }},
	{"still_stable_every_hours", MotionParamInt, func(p *MotionParams) any { return &p.StillStableEveryHours }},
	{"trend_deg_7d", MotionParamFloat, func(p *MotionParams) any { return &p.TrendDeg7d }},
}

// MotionParamFields devuelve la tabla de parámetros (copia).
func MotionParamFields() []MotionParamField {
	return append([]MotionParamField(nil), motionParamFields...)
}

// LookupMotionParam busca un parámetro por nombre.
func LookupMotionParam(name string) (MotionParamField, bool) {
	for _, f := range motionParamFields {
		if f.Name == name {
			return f, true
		}
	}
	return MotionParamField{}, false
}

// Get devuelve el valor del parámetro en p (float64, int, bool o TempMode).
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
	}
	return nil
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
	}
	return false
}

// MotionPatch son los parámetros informados de una versión nueva, por nombre
// (los demás se copian de la vigente). Los valores son float64, int, bool o
// TempMode según el parámetro.
type MotionPatch map[string]any

// ApplyTo copia de base lo que el parche no informa. Un nombre desconocido o
// un valor de otro tipo es ErrInvalidPatch.
func (patch MotionPatch) ApplyTo(base MotionParams) (MotionParams, error) {
	p := base
	for name, v := range patch {
		f, ok := LookupMotionParam(name)
		if !ok || !f.set(&p, v) {
			return MotionParams{}, domain.ErrInvalidPatch
		}
	}
	return p, nil
}

// Normalize redondea grados y °C/h a la precisión de la columna (4
// decimales) y normaliza temp_mode. Se valida DESPUÉS de normalizar.
func (p MotionParams) Normalize() MotionParams {
	for _, f := range motionParamFields {
		if f.Kind == MotionParamFloat {
			v := f.ptr(&p).(*float64)
			*v = round4(*v)
		}
	}
	p.TempMode = TempMode(strings.ToUpper(strings.TrimSpace(string(p.TempMode))))
	return p
}

// Validate aplica las mismas reglas que los CHECK de motion_threshold_versions.
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
		}
	}
	if p.MinRecordsHour > p.RecordsPerHour {
		return domain.ErrMotionRecordsRange
	}
	return nil
}

// Window devuelve la ventana de registros crudos que necesita la hora t: [t − 169 h, t).
func MotionWindow(t time.Time) (time.Time, time.Time) {
	return t.Add(-MotionWindowHours * time.Hour), t
}

// MotionWindowHours es el largo de la ventana que necesita MotionAssess:
// [t − 169 h, t) alcanza para el nivel de t − 168 h (trend_flag).
const MotionWindowHours = 169

// MotionPilotPointID es el punto de prueba (MP-11556): el seed le da la v1 de
// §5.2; al resto, la de punto nuevo (modo sombra, SOLO_MOVIMIENTO).
const MotionPilotPointID int64 = 11556

// IsEffectiveAt indica si la versión aplica a la hora t.
func (v MotionThresholdVersion) IsEffectiveAt(t time.Time) bool {
	return !v.ValidFrom.After(t) && (v.ValidTo == nil || v.ValidTo.After(t))
}
