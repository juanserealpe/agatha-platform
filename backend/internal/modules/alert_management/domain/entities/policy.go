package entities

import (
	"math"
	"time"
)

// PolicyParams son los parámetros de Alert Management (alert_policy_versions).
// Los de abrir y cerrar (2 h sobre 0,16 / 2 h bajo 0,14) son del Rule Engine.
type PolicyParams struct {
	// DowngradeHours: horas calculadas seguidas por debajo del nivel actual que lo bajan un nivel (D-34).
	DowngradeHours int
	// RojoRequiresClassificationToDowngrade: el rojo sin clasificar no baja (D-35).
	RojoRequiresClassificationToDowngrade bool
	// MergeWindowHours: ventana de reactivación (D-31).
	MergeWindowHours int
	// ReactivateOnlyIfUnclassified: una alerta clasificada no se reactiva.
	ReactivateOnlyIfUnclassified bool
	// InterventionSilentMaxLevel: mayor nivel que sube sin notificar con la
	// etiqueta de intervención (nunca ROJO, D-36).
	InterventionSilentMaxLevel Level
	SummaryEveryHours          int
	CountdownHours             int
	TrendLookbackHours         int
	TrendDelta                 float64
	OscillationFlips           int
	OscillationWindowHours     int
	// LateNotificationMaxHours: una hora con más atraso se publica marcada
	// Late (la especificación pide no enviar push; lo aplica Notification).
	LateNotificationMaxHours int
}

// DefaultPolicyParams es la versión global inicial (todos pendientes de validación, sección 11).
func DefaultPolicyParams() PolicyParams {
	return PolicyParams{
		DowngradeHours:                        6,
		RojoRequiresClassificationToDowngrade: true,
		MergeWindowHours:                      24,
		ReactivateOnlyIfUnclassified:          true,
		InterventionSilentMaxLevel:            LevelNaranja,
		SummaryEveryHours:                     6,
		CountdownHours:                        3,
		TrendLookbackHours:                    3,
		TrendDelta:                            0.02,
		OscillationFlips:                      4,
		OscillationWindowHours:                6,
		LateNotificationMaxHours:              2,
	}
}

// Normalize redondea trend_delta a la precisión de la columna (4 decimales).
func (p PolicyParams) Normalize() PolicyParams {
	p.TrendDelta = math.Round(p.TrendDelta*1e4) / 1e4
	return p
}

// Validate aplica las mismas reglas que los CHECK de la tabla.
func (p PolicyParams) Validate() bool {
	for _, h := range []int{p.DowngradeHours, p.MergeWindowHours, p.SummaryEveryHours, p.CountdownHours,
		p.TrendLookbackHours, p.OscillationFlips, p.OscillationWindowHours} {
		if h < 1 {
			return false
		}
	}
	if p.LateNotificationMaxHours < 0 || p.TrendDelta <= 0 || p.TrendDelta >= 1 {
		return false
	}
	return p.InterventionSilentMaxLevel == LevelAmarillo || p.InterventionSilentMaxLevel == LevelNaranja
}

// PolicyPatch son los campos que cambian en una versión nueva (nil = se copia de la vigente).
type PolicyPatch struct {
	DowngradeHours                        *int
	RojoRequiresClassificationToDowngrade *bool
	MergeWindowHours                      *int
	ReactivateOnlyIfUnclassified          *bool
	InterventionSilentMaxLevel            *Level
	SummaryEveryHours                     *int
	CountdownHours                        *int
	TrendLookbackHours                    *int
	TrendDelta                            *float64
	OscillationFlips                      *int
	OscillationWindowHours                *int
	LateNotificationMaxHours              *int
}

// Apply copia sobre base los campos informados.
func (patch PolicyPatch) Apply(base PolicyParams) PolicyParams {
	setInt := func(dst *int, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	setBool := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	setInt(&base.DowngradeHours, patch.DowngradeHours)
	setBool(&base.RojoRequiresClassificationToDowngrade, patch.RojoRequiresClassificationToDowngrade)
	setInt(&base.MergeWindowHours, patch.MergeWindowHours)
	setBool(&base.ReactivateOnlyIfUnclassified, patch.ReactivateOnlyIfUnclassified)
	if patch.InterventionSilentMaxLevel != nil {
		base.InterventionSilentMaxLevel = *patch.InterventionSilentMaxLevel
	}
	setInt(&base.SummaryEveryHours, patch.SummaryEveryHours)
	setInt(&base.CountdownHours, patch.CountdownHours)
	setInt(&base.TrendLookbackHours, patch.TrendLookbackHours)
	if patch.TrendDelta != nil {
		base.TrendDelta = *patch.TrendDelta
	}
	setInt(&base.OscillationFlips, patch.OscillationFlips)
	setInt(&base.OscillationWindowHours, patch.OscillationWindowHours)
	setInt(&base.LateNotificationMaxHours, patch.LateNotificationMaxHours)
	return base
}

// Policy es una versión de la política. MeasuringPointID nil = global; una
// versión de un punto reemplaza a la global en ese punto. Solo se agregan
// versiones, sin retroactividad (patrón de rule_threshold_versions).
type Policy struct {
	ID               int64
	Version          int
	MeasuringPointID *int64
	EventType        EventType
	PolicyParams
	ValidFrom time.Time
	ValidTo   *time.Time
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// Valores del seed de la versión global.
const (
	SystemCreatedBy  = "system"
	PolicySeedReason = "versión global inicial (especificación v3.3)"
)
