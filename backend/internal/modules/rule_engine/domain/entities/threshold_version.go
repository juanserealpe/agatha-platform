package entities

import (
	"math"
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
}

// DefaultThresholdParams es la plantilla de configuración (todos los valores
// son beta). La usan el seed y la primera versión de un punto nuevo.
func DefaultThresholdParams() ThresholdParams {
	return ThresholdParams{
		Amarillo:                   0.18,
		Naranja:                    0.25,
		Rojo:                       0.35,
		MinHours:                   2,
		EscalateAmarilloAfterHours: intPtr(12),
		EscalateNaranjaAfterHours:  intPtr(6),
		ExitMargin:                 0.02,
		ExitHours:                  2,
	}
}

// Normalize redondea los umbrales y el margen a la precisión de la columna
// (4 decimales). Se valida DESPUÉS de normalizar: dos valores distintos pueden
// quedar iguales al redondear.
func (p ThresholdParams) Normalize() ThresholdParams {
	p.Amarillo = round4(p.Amarillo)
	p.Naranja = round4(p.Naranja)
	p.Rojo = round4(p.Rojo)
	p.ExitMargin = round4(p.ExitMargin)
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
	return nil
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
	}
}

// ThresholdVersion es una fila de rule_threshold_versions: inmutable salvo el
// cierre de la vigente (ValidTo) cuando se crea la siguiente.
type ThresholdVersion struct {
	ID               int64
	MeasuringPointID int64
	EventType        EventType
	Version          int
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

func intPtr(v int) *int { return &v }

func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	return intPtr(*p)
}
