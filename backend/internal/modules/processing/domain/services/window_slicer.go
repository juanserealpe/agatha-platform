package services

import (
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// SliceWindow devuelve la sub-secuencia [t − windowHours, t) de samples
// (ordenados por T, p. ej. tras features.Normalize). No copia: comparte el
// arreglo subyacente. Un sample exactamente en t pertenece a la ventana siguiente.
func SliceWindow(samples []features.Sample, t time.Time, windowHours int) []features.Sample {
	from := t.Add(-time.Duration(windowHours) * time.Hour)

	lo := sort.Search(len(samples), func(i int) bool { return !samples[i].T.Before(from) })
	hi := sort.Search(len(samples), func(i int) bool { return !samples[i].T.Before(t) })

	return samples[lo:hi]
}
