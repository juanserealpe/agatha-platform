package services

import (
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// ExclusionPolicy convierte marcas horarias en periodos excluidos con tope
// sobre la duración del histórico (sección 9.4).
type ExclusionPolicy struct {
	capRatio float64
}

func NewExclusionPolicy(cfg Config) ExclusionPolicy {
	return ExclusionPolicy{capRatio: cfg.ExclusionCapRatio}
}

// ApplyWithKnown: cada marca t ⇒ [t − 24 h, t); se agregan por I descendente
// (empate: t ascendente) mientras la unión sea ≤ tope × (to − from). En la
// primera marca que lo superaría se detiene y marca CapReached. Los marks
// deben ser horas alineadas en [from + 24 h, to].
//
// Los eventos conocidos se excluyen SIEMPRE (recortados a [from, to)), se unen
// a los periodos de las marcas y sus horas cuentan para el tope, de modo que
// las marcas solo disponen del resto. MarkedHours contiene solo las marcas
// aceptadas (criterio de convergencia). Un periodo que solo proviene de un
// evento tiene PeakI = 0.
func (p ExclusionPolicy) ApplyWithKnown(
	marks []HourMark,
	known []entities.KnownEvent,
	from, to time.Time,
) entities.ExclusionSet {
	ordered := make([]HourMark, len(marks))
	copy(ordered, marks)
	sort.SliceStable(ordered, func(a, b int) bool {
		if ordered[a].I != ordered[b].I {
			return ordered[a].I > ordered[b].I
		}
		return ordered[a].T.Before(ordered[b].T)
	})

	capDuration := time.Duration(p.capRatio * float64(to.Sub(from)))

	// Las marcas son horas alineadas: la unión se lleva como conjunto de horas cubiertas.
	covered := map[int64]bool{}
	forced := clipKnownEvents(known, from, to)
	for _, k := range forced {
		for h := k.From.Truncate(time.Hour); h.Before(k.To); h = h.Add(time.Hour) {
			covered[h.Unix()] = true
		}
	}

	accepted := []HourMark{}
	capReached := false

	for _, m := range ordered {
		newHours := []int64{}
		for h := m.T.Add(-scanWindowHours * time.Hour); h.Before(m.T); h = h.Add(time.Hour) {
			if key := h.Unix(); !covered[key] {
				newHours = append(newHours, key)
			}
		}

		if time.Duration(len(covered)+len(newHours))*time.Hour > capDuration {
			capReached = true
			break
		}

		for _, key := range newHours {
			covered[key] = true
		}
		accepted = append(accepted, m)
	}

	return entities.ExclusionSet{
		Periods:     unionPeriods(mergePeriods(accepted), forced),
		MarkedHours: sortedHours(accepted),
		CapReached:  capReached,
	}
}

// mergePeriods fusiona los intervalos [t − 24 h, t) que se solapan o tocan;
// PeakI es el máximo I de las marcas de cada periodo.
func mergePeriods(marks []HourMark) []entities.ExcludedPeriod {
	ordered := make([]HourMark, len(marks))
	copy(ordered, marks)
	sort.Slice(ordered, func(a, b int) bool { return ordered[a].T.Before(ordered[b].T) })

	periods := []entities.ExcludedPeriod{}
	for _, m := range ordered {
		from := m.T.Add(-scanWindowHours * time.Hour)
		last := len(periods) - 1
		if last >= 0 && !from.After(periods[last].To) {
			if m.T.After(periods[last].To) {
				periods[last].To = m.T
			}
			if m.I > periods[last].PeakI {
				periods[last].PeakI = m.I
			}
			continue
		}
		periods = append(periods, entities.ExcludedPeriod{From: from, To: m.T, PeakI: m.I})
	}

	return periods
}

// clipKnownEvents recorta los eventos a [from, to) y descarta los que quedan vacíos.
func clipKnownEvents(known []entities.KnownEvent, from, to time.Time) []entities.ExcludedPeriod {
	out := []entities.ExcludedPeriod{}
	for _, k := range known {
		start, end := k.From.UTC(), k.To.UTC()
		if start.Before(from.UTC()) {
			start = from.UTC()
		}
		if end.After(to.UTC()) {
			end = to.UTC()
		}
		if start.Before(end) {
			out = append(out, entities.ExcludedPeriod{From: start, To: end})
		}
	}
	return out
}

// unionPeriods une dos listas de periodos fusionando los que se solapan o
// tocan; PeakI es el máximo de los fusionados.
func unionPeriods(periods, extra []entities.ExcludedPeriod) []entities.ExcludedPeriod {
	if len(extra) == 0 {
		return periods
	}

	all := append(append([]entities.ExcludedPeriod{}, periods...), extra...)
	sort.SliceStable(all, func(a, b int) bool { return all[a].From.Before(all[b].From) })

	out := []entities.ExcludedPeriod{}
	for _, p := range all {
		last := len(out) - 1
		if last >= 0 && !p.From.After(out[last].To) {
			if p.To.After(out[last].To) {
				out[last].To = p.To
			}
			if p.PeakI > out[last].PeakI {
				out[last].PeakI = p.PeakI
			}
			continue
		}
		out = append(out, p)
	}

	return out
}

func sortedHours(marks []HourMark) []time.Time {
	hours := make([]time.Time, len(marks))
	for i, m := range marks {
		hours[i] = m.T
	}
	sort.Slice(hours, func(a, b int) bool { return hours[a].Before(hours[b]) })
	return hours
}

// ExcludedRatio es la fracción de [from, to) cubierta por la exclusión.
func ExcludedRatio(e entities.ExclusionSet, from, to time.Time) float64 {
	total := to.Sub(from)
	if total <= 0 {
		return 0
	}
	return float64(e.Duration()) / float64(total)
}
