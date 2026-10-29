package entities

import "time"

// ExcludedPeriod es un intervalo [From, To) excluido del cálculo: por anómalo
// (barrido de I) o por una alerta.
type ExcludedPeriod struct {
	From time.Time
	To   time.Time
}

// Contains indica si t ∈ [From, To).
func (p ExcludedPeriod) Contains(t time.Time) bool {
	return !t.Before(p.From) && t.Before(p.To)
}

// Overlaps indica si [from, to) se solapa con [From, To).
func (p ExcludedPeriod) Overlaps(from, to time.Time) bool {
	return from.Before(p.To) && p.From.Before(to)
}

// ExclusionSet es el conjunto excluido con el que se calcula una línea base.
// Periods está ordenado y sin solapes; MarkedHours son las horas marcadas
// aceptadas (tras aplicar el tope), ordenadas.
type ExclusionSet struct {
	Periods     []ExcludedPeriod
	MarkedHours []time.Time
	CapReached  bool
}

// Excludes indica si t cae dentro de algún periodo excluido.
func (e ExclusionSet) Excludes(t time.Time) bool {
	for _, p := range e.Periods {
		if p.Contains(t) {
			return true
		}
	}
	return false
}

// OverlapsRange indica si [from, to) se solapa con algún periodo excluido.
func (e ExclusionSet) OverlapsRange(from, to time.Time) bool {
	for _, p := range e.Periods {
		if p.Overlaps(from, to) {
			return true
		}
	}
	return false
}

// Duration es la duración total de la unión de periodos.
func (e ExclusionSet) Duration() time.Duration {
	var d time.Duration
	for _, p := range e.Periods {
		d += p.To.Sub(p.From)
	}
	return d
}

// SameMarks compara los conjuntos de horas marcadas (criterio de convergencia).
func (e ExclusionSet) SameMarks(other ExclusionSet) bool {
	if len(e.MarkedHours) != len(other.MarkedHours) {
		return false
	}
	for i := range e.MarkedHours {
		if !e.MarkedHours[i].Equal(other.MarkedHours[i]) {
			return false
		}
	}
	return true
}
