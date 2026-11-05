package features

import (
	"math"
	"sort"
	"time"
)

// Sample es un registro del dispositivo con las tres series que usa el kernel.
// Has* = false si el valor está ausente o no es finito.
type Sample struct {
	T                   time.Time
	B1, B2, Pl          float64
	HasB1, HasB2, HasPl bool
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// sanitize marca como ausentes los valores no finitos.
func sanitize(s Sample) Sample {
	s.HasB1 = s.HasB1 && isFinite(s.B1)
	s.HasB2 = s.HasB2 && isFinite(s.B2)
	s.HasPl = s.HasPl && isFinite(s.Pl)
	return s
}

// SlotOf devuelve el inicio del slot de 15 min (UTC) que contiene t.
func SlotOf(t time.Time) time.Time {
	return t.UTC().Truncate(Slot)
}

// Normalize ordena por T y deja un sample por slot floor(T, 15m): conserva el
// que tenga B2 válido; si ambos (o ninguno) lo tienen, el más temprano.
// Los valores NaN/±Inf se marcan como ausentes. No muta la entrada.
func Normalize(in []Sample) []Sample {
	if len(in) == 0 {
		return []Sample{}
	}

	sorted := make([]Sample, len(in))
	for i, s := range in {
		sorted[i] = sanitize(s)
	}
	sort.SliceStable(sorted, func(a, b int) bool { return sorted[a].T.Before(sorted[b].T) })

	out := make([]Sample, 0, len(sorted))
	for _, s := range sorted {
		last := len(out) - 1
		if last >= 0 && SlotOf(out[last].T).Equal(SlotOf(s.T)) {
			// Mismo slot: solo reemplaza si el actual no tiene B2 y el nuevo sí.
			if !out[last].HasB2 && s.HasB2 {
				out[last] = s
			}
			continue
		}
		out = append(out, s)
	}

	return out
}
