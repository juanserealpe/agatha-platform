package services

import (
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// WindowSpec define una familia de ventanas históricas.
type WindowSpec struct {
	Hours int
	Step  time.Duration
}

// HistoricalWindows: 6 h con fin cada 3 h; 24 h con fin cada 6 h (alineadas a UTC).
var HistoricalWindows = []WindowSpec{
	{Hours: 6, Step: 3 * time.Hour},
	{Hours: 24, Step: 6 * time.Hour},
}

// HistoricalWindowGenerator produce los fines de ventana alineados en UTC.
type HistoricalWindowGenerator struct{}

// Ends devuelve los fines alineados a step (00, 03, … / 00, 06, 12, 18 UTC)
// tales que la ventana completa [end − hours, end) cae dentro de [from, to].
func (HistoricalWindowGenerator) Ends(from, to time.Time, spec WindowSpec) []time.Time {
	length := time.Duration(spec.Hours) * time.Hour
	first := ceilTo(from.UTC().Add(length), spec.Step)

	ends := []time.Time{}
	for end := first; !end.After(to.UTC()); end = end.Add(spec.Step) {
		ends = append(ends, end)
	}

	return ends
}

// ceilTo redondea t hacia arriba al múltiplo de step (UTC).
func ceilTo(t time.Time, step time.Duration) time.Time {
	floor := t.Truncate(step)
	if floor.Before(t) {
		return floor.Add(step)
	}
	return floor
}

// samplesIn devuelve la subsecuencia de samples (ordenados por T) en [from, to).
func samplesIn(samples []features.Sample, from, to time.Time) []features.Sample {
	lo := sort.Search(len(samples), func(i int) bool { return !samples[i].T.Before(from) })
	hi := sort.Search(len(samples), func(i int) bool { return !samples[i].T.Before(to) })
	return samples[lo:hi]
}
