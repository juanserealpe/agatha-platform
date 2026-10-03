package services

import (
	"math/rand/v2"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

var day0 = time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

// episode es un tramo anómalo: B2 alto y B1/Planitud acoplados a B2.
type episode struct {
	from, to time.Time
}

// synthetic genera samples cada 15 min en [from, from + days) con ruido
// determinista (semilla fija). Fuera de episodios B1, B2 y Planitud son
// independientes; dentro, B2 sube y las tres series se acoplan.
func synthetic(from time.Time, days int, episodes ...episode) []features.Sample {
	rng := rand.New(rand.NewPCG(42, 7))
	n := days * 96
	out := make([]features.Sample, 0, n)

	for i := 0; i < n; i++ {
		t := from.Add(time.Duration(i) * features.Slot)
		b2 := float64(40 + rng.IntN(31)) // 40..70
		b1 := float64(rng.IntN(100))
		pl := rng.Float64()

		for _, ep := range episodes {
			if !t.Before(ep.from) && t.Before(ep.to) {
				step := float64(t.Sub(ep.from) / features.Slot)
				b2 = 120 + step
				b1 = 200 + step
				pl = 2 + step/100
			}
		}

		out = append(out, features.Sample{T: t, B1: b1, B2: b2, Pl: pl, HasB1: true, HasB2: true, HasPl: true})
	}

	return out
}

func approxEq(a, b float64) bool {
	d := a - b
	return d < 1e-9 && d > -1e-9
}
