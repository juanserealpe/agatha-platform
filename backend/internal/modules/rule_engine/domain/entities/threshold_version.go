package entities

import (
	"math"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
)

// ThresholdParams son los parámetros de una versión de umbrales. Los umbrales
// se guardan como NUMERIC(5,4): Normalize los redondea a 4 decimales.
type ThresholdParams struct {
	Amarillo float64
	Naranja  float64
	Rojo     float64
	// MinHours son las horas calculadas consecutivas con raw >= AMARILLO que
	// abren un episodio.
	MinHours int
	// EscalateAmarilloAfterHours: racha con raw >= AMARILLO que sube a NARANJA
	// por persistencia (nil = sin escalamiento).
	EscalateAmarilloAfterHours *int
	// EscalateNaranjaAfterHours: racha con raw >= NARANJA que sube a ROJO por
	// persistencia (nil = sin escalamiento).
	EscalateNaranjaAfterHours *int
	// ExitMargin: el episodio cierra con I < Amarillo − ExitMargin (histéresis).
	ExitMargin float64
	// ExitHours: horas calculadas consecutivas bajo la banda de salida que cierran el episodio.
	ExitHours int

	// ConfirmNaranjaWithPlanitud: NARANJA (por umbral o persistencia) solo se
	// asigna si el d_pl de la hora es >= DPlMin; si no, la hora queda en
	// AMARILLO con el motivo NARANJA_NOT_CONFIRMED.
	ConfirmNaranjaWithPlanitud bool
	// DPlMin es el % mínimo de registros con planitud alta (0–100). Obligatorio
	// si ConfirmNaranjaWithPlanitud.
	DPlMin *float64
	// PersistenceMode: STRICT = racha seguida; WINDOW = horas sobre el corte en
	// las últimas PersistenceWindowHours horas calculadas.
	PersistenceMode PersistenceMode
	// PersistenceWindowHours (solo WINDOW): tamaño de la ventana móvil.
	PersistenceWindowHours *int
	// PersistenceMinHours (solo WINDOW): horas sobre el corte dentro de la
	// ventana que escalan un nivel (reemplaza a escalate_*_after_hours como
	// umbral; un escalamiento NULL sigue desactivado).
	PersistenceMinHours *int
}

// DefaultThresholdParams es la plantilla de configuración (todos los valores
// son beta). La usan el seed y la primera versión de un punto nuevo. Amarillo
// 0,16 (salida 0,14) es la v2 de la especificación: +2 h de antelación en la
// fuga del 24 ago a costa de 1 amarillo sin evento (26 sep).
func DefaultThresholdParams() ThresholdParams {
	return ThresholdParams{
		Amarillo:                   0.16,
		Naranja:                    0.25,
		Rojo:                       0.35,
		MinHours:                   2,
		EscalateAmarilloAfterHours: intPtr(12),
		EscalateNaranjaAfterHours:  intPtr(6),
		ExitMargin:                 0.02,
		ExitHours:                  2,
		PersistenceMode:            PersistenceStrict,
	}
}

// Normalize redondea los umbrales y el margen a la precisión de la columna
// (4 decimales; d_pl_min a 2) y normaliza el modo de persistencia (vacío ⇒ STRICT). Se valida DESPUÉS de normalizar: dos valores distintos pueden
// quedar iguales al redondear.
func (p ThresholdParams) Normalize() ThresholdParams {
	p.Amarillo = round4(p.Amarillo)
	p.Naranja = round4(p.Naranja)
	p.Rojo = round4(p.Rojo)
	p.ExitMargin = round4(p.ExitMargin)
	if p.DPlMin != nil {
		p.DPlMin = floatPtr(round2(*p.DPlMin))
	}
	p.PersistenceMode = PersistenceMode(strings.ToUpper(strings.TrimSpace(string(p.PersistenceMode))))
	if p.PersistenceMode == "" {
		p.PersistenceMode = PersistenceStrict
	}
	return p
}

// Validate aplica las mismas reglas que los CHECK de rule_threshold_versions.
// Devuelve el primer error de validación (*AppError de domain).
func (p ThresholdParams) Validate() error {
	if !(p.Amarillo > 0 && p.Amarillo < p.Naranja && p.Naranja < p.Rojo && p.Rojo <= 1) {
		return domain.ErrThresholdOrder
	}
	if !(p.ExitMargin >= 0 && p.ExitMargin < p.Amarillo) {
		return domain.ErrExitMarginRange
	}
	if p.MinHours < 1 || p.ExitHours < 1 {
		return domain.ErrHoursRange
	}
	if (p.EscalateAmarilloAfterHours != nil && *p.EscalateAmarilloAfterHours < 1) ||
		(p.EscalateNaranjaAfterHours != nil && *p.EscalateNaranjaAfterHours < 1) {
		return domain.ErrHoursRange
	}
	if p.DPlMin != nil && !(*p.DPlMin >= 0 && *p.DPlMin <= 100) {
		return domain.ErrPlanitudConfig
	}
	if p.ConfirmNaranjaWithPlanitud && p.DPlMin == nil {
		return domain.ErrPlanitudConfig
	}
	return p.validatePersistence()
}

// validatePersistence: el modo es STRICT o WINDOW; las horas de ventana, si
// vienen, >= 1; con WINDOW ambas son obligatorias y min <= ventana.
func (p ThresholdParams) validatePersistence() error {
	if p.PersistenceMode != PersistenceStrict && p.PersistenceMode != PersistenceWindow {
		return domain.ErrPersistenceConfig
	}
	if (p.PersistenceWindowHours != nil && *p.PersistenceWindowHours < 1) ||
		(p.PersistenceMinHours != nil && *p.PersistenceMinHours < 1) {
		return domain.ErrPersistenceConfig
	}
	if p.PersistenceMode == PersistenceWindow &&
		(p.PersistenceWindowHours == nil || p.PersistenceMinHours == nil || *p.PersistenceMinHours > *p.PersistenceWindowHours) {
		return domain.ErrPersistenceConfig
	}
	return nil
}

// IsWindow indica si la persistencia se cuenta por ventana móvil.
func (p ThresholdParams) IsWindow() bool { return p.PersistenceMode == PersistenceWindow }

// EscalationHours es el umbral de horas que escala desde from (AMARILLO →
// NARANJA, NARANJA → ROJO) con el modo vigente: escalate_*_after_hours en
// STRICT, persistence_min_hours en WINDOW. nil = escalamiento desactivado.
func (p ThresholdParams) EscalationHours(from Level) *int {
	var configured *int
	switch from {
	case LevelAmarillo:
		configured = p.EscalateAmarilloAfterHours
	case LevelNaranja:
		configured = p.EscalateNaranjaAfterHours
	}
	if configured == nil {
		return nil
	}
	if p.IsWindow() {
		return copyIntPtr(p.PersistenceMinHours)
	}
	return copyIntPtr(configured)
}

// IsNaranjaConfirmed indica si el d_pl de la hora confirma NARANJA. Sin la
// confirmación activa, siempre; con ella, d_pl conocido y >= d_pl_min.
func (p ThresholdParams) IsNaranjaConfirmed(dPl *float64) bool {
	if !p.ConfirmNaranjaWithPlanitud {
		return true
	}
	return dPl != nil && p.DPlMin != nil && *dPl >= *p.DPlMin
}

// HistoryDepth es K: cuántas horas calculadas anteriores hacen falta para
// evaluar una hora (el mayor parámetro de horas + 1).
func (p ThresholdParams) HistoryDepth() int {
	k := max(p.MinHours, p.ExitHours)
	if p.EscalateAmarilloAfterHours != nil {
		k = max(k, *p.EscalateAmarilloAfterHours)
	}
	if p.EscalateNaranjaAfterHours != nil {
		k = max(k, *p.EscalateNaranjaAfterHours)
	}
	if p.IsWindow() && p.PersistenceWindowHours != nil {
		k = max(k, *p.PersistenceWindowHours)
	}
	return k + 1
}

// ExitCutoff es el valor bajo el cual una hora cuenta para cerrar el episodio.
// Se redondea a 4 decimales para que 0,18 − 0,02 sea exactamente 0,16.
func (p ThresholdParams) ExitCutoff() float64 { return p.ExitFor(LevelAmarillo) }

// CutFor es el corte de I de un nivel (0 para NORMAL).
func (p ThresholdParams) CutFor(l Level) float64 {
	switch l {
	case LevelAmarillo:
		return p.Amarillo
	case LevelNaranja:
		return p.Naranja
	case LevelRojo:
		return p.Rojo
	default:
		return 0
	}
}

// ExitFor es la salida (histéresis) de un nivel: su corte − exit_margin,
// redondeado a 4 decimales (0 para NORMAL). Con la plantilla: 0,16 / 0,23 / 0,33.
func (p ThresholdParams) ExitFor(l Level) float64 {
	if l.Rank() == 0 {
		return 0
	}
	return round4(p.CutFor(l) - p.ExitMargin)
}

// Optional es un campo de un parche: Set = false ⇒ se copia de la versión vigente.
type Optional[T any] struct {
	Set   bool
	Value T
}

// Some marca un campo del parche como informado.
func Some[T any](v T) Optional[T] { return Optional[T]{Set: true, Value: v} }

func (o Optional[T]) or(current T) T {
	if o.Set {
		return o.Value
	}
	return current
}

// ThresholdPatch son los parámetros parciales de una versión nueva. Para los
// escalamientos, Some[*int](nil) los deja en NULL (sin escalamiento).
type ThresholdPatch struct {
	Amarillo                   Optional[float64]
	Naranja                    Optional[float64]
	Rojo                       Optional[float64]
	MinHours                   Optional[int]
	EscalateAmarilloAfterHours Optional[*int]
	EscalateNaranjaAfterHours  Optional[*int]
	ExitMargin                 Optional[float64]
	ExitHours                  Optional[int]
	ConfirmNaranjaWithPlanitud Optional[bool]
	DPlMin                     Optional[*float64]
	PersistenceMode            Optional[PersistenceMode]
	PersistenceWindowHours     Optional[*int]
	PersistenceMinHours        Optional[*int]
}

// ApplyTo copia de base los campos que el parche no informa.
func (patch ThresholdPatch) ApplyTo(base ThresholdParams) ThresholdParams {
	return ThresholdParams{
		Amarillo:                   patch.Amarillo.or(base.Amarillo),
		Naranja:                    patch.Naranja.or(base.Naranja),
		Rojo:                       patch.Rojo.or(base.Rojo),
		MinHours:                   patch.MinHours.or(base.MinHours),
		EscalateAmarilloAfterHours: copyIntPtr(patch.EscalateAmarilloAfterHours.or(base.EscalateAmarilloAfterHours)),
		EscalateNaranjaAfterHours:  copyIntPtr(patch.EscalateNaranjaAfterHours.or(base.EscalateNaranjaAfterHours)),
		ExitMargin:                 patch.ExitMargin.or(base.ExitMargin),
		ExitHours:                  patch.ExitHours.or(base.ExitHours),
		ConfirmNaranjaWithPlanitud: patch.ConfirmNaranjaWithPlanitud.or(base.ConfirmNaranjaWithPlanitud),
		DPlMin:                     copyFloatPtr(patch.DPlMin.or(base.DPlMin)),
		PersistenceMode:            patch.PersistenceMode.or(base.PersistenceMode),
		PersistenceWindowHours:     copyIntPtr(patch.PersistenceWindowHours.or(base.PersistenceWindowHours)),
		PersistenceMinHours:        copyIntPtr(patch.PersistenceMinHours.or(base.PersistenceMinHours)),
	}
}

// ThresholdVersion es una fila de rule_threshold_versions: inmutable salvo el
// cierre de la vigente (ValidTo) cuando se crea la siguiente.
type ThresholdVersion struct {
	ID               int64
	MeasuringPointID int64
	EventType        EventType
	Version          int
	// PreviousVersionID es el ID de la versión anterior del punto (nil en la
	// 1). No es columna: lo completa el caso de uso al crear o consultar.
	PreviousVersionID *int64
	ThresholdParams
	ValidFrom time.Time
	ValidTo   *time.Time // nil = vigente
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// IsEffectiveAt indica si la versión aplica a la hora t:
// ValidFrom <= t AND (ValidTo IS NULL OR ValidTo > t).
func (v ThresholdVersion) IsEffectiveAt(t time.Time) bool {
	return !v.ValidFrom.After(t) && (v.ValidTo == nil || v.ValidTo.After(t))
}

// Seed: valores de la versión 1 que crea el sistema.
const (
	SystemCreatedBy    = "system"
	SeedReason         = "valor inicial beta"
	FirstUseSeedReason = "valor inicial beta (primer uso)"
)

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

func round2(v float64) float64 { return math.Round(v*1e2) / 1e2 }

func floatPtr(v float64) *float64 { return &v }

func copyFloatPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	return floatPtr(*p)
}

func intPtr(v int) *int { return &v }

func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	return intPtr(*p)
}
