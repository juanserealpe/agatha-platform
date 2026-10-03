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

// Apply: cada marca t ⇒ [t − 24 h, t); se agregan por I descendente (empate: t
// ascendente) mientras la unión sea ≤ tope × (to − from). En la primera marca
// que lo superaría se detiene y marca CapReached. Los marks deben ser horas
// alineadas en [from + 24 h, to].
func (p ExclusionPolicy) Apply(marks []HourMark, from, to time.Time) entities.ExclusionSet {
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
		Periods:     mergePeriods(accepted),
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
