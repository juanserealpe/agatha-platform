package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

const scanWindowHours = 24

// HourMark es una hora cuyo I(24 h) superó el corte de exclusión.
type HourMark struct {
	T time.Time // fin de la ventana [T − 24 h, T)
	I float64
}

// HourlyAnomalyScanner calcula I(24 h) de cada hora con una línea base dada y
// marca las anómalas (sección 9.3).
type HourlyAnomalyScanner struct {
	cutoff float64
}

func NewHourlyAnomalyScanner(cfg Config) HourlyAnomalyScanner {
	return HourlyAnomalyScanner{cutoff: cfg.ExclusionICutoff}
}

// Scan recorre cada hora alineada t ∈ [from + 24 h, to] con la ventana
// [t − 24 h, t) sobre TODOS los samples (incluidos los excluidos). samples debe
// estar normalizado. Marca si I ≥ corte (sin exigir horas consecutivas).
func (s HourlyAnomalyScanner) Scan(
	samples []features.Sample,
	sig features.Signal,
	stats24 features.ComponentStats,
	from, to time.Time,
) []HourMark {
	marks := []HourMark{}

	eachScanWindow(samples, from, to, func(t time.Time, window []features.Sample) {
		comp := features.ComputeComponents(window, scanWindowHours, sig, true)
		if !comp.Calculable || !comp.HasC {
			return
		}

		scores, ok := features.Score(comp, stats24, scanWindowHours, sig)
		if !ok {
			return
		}

		if scores.I >= s.cutoff {
			marks = append(marks, HourMark{T: t, I: scores.I})
		}
	})

	return marks
}

// eachScanWindow llama fn con cada hora del barrido y los samples de su
// ventana [t − 24 h, t), recorridos con dos punteros.
func eachScanWindow(samples []features.Sample, from, to time.Time, fn func(t time.Time, window []features.Sample)) {
	length := scanWindowHours * time.Hour

	// Primera hora alineada ≥ from + 24 h: su ventana [from, from + 24 h) está
	// completa dentro del rango.
	earliest := from.UTC().Add(length)
	first := earliest.Truncate(time.Hour)
	if first.Before(earliest) {
		first = first.Add(time.Hour)
	}

	// Dos punteros: [lo, hi) son los samples de la ventana actual.
	lo, hi := 0, 0
	for t := first; !t.After(to.UTC()); t = t.Add(time.Hour) {
		start := t.Add(-length)
		for hi < len(samples) && samples[hi].T.Before(t) {
			hi++
		}
		for lo < hi && samples[lo].T.Before(start) {
			lo++
		}

		fn(t, samples[lo:hi])
	}
}
