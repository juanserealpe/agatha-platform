package features

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

func TestNormalize(t *testing.T) {
	t.Run("ordena muestras desordenadas", func(t *testing.T) {
		in := []Sample{
			{T: t0.Add(30 * time.Minute), B2: 3, HasB2: true},
			{T: t0, B2: 1, HasB2: true},
			{T: t0.Add(15 * time.Minute), B2: 2, HasB2: true},
		}
		out := Normalize(in)
		for i, want := range []float64{1, 2, 3} {
			if out[i].B2 != want {
				t.Fatalf("out[%d].B2 = %v, se esperaba %v", i, out[i].B2, want)
			}
		}
	})

	t.Run("mismo slot conserva el que tiene B2 válido", func(t *testing.T) {
		in := []Sample{
			{T: t0.Add(1 * time.Minute), B1: 9, HasB1: true},
			{T: t0.Add(5 * time.Minute), B2: 7, HasB2: true},
		}
		out := Normalize(in)
		if len(out) != 1 || !out[0].HasB2 || out[0].B2 != 7 {
			t.Fatalf("out = %+v, se esperaba el sample con B2", out)
		}
	})

	t.Run("mismo slot con ambos válidos conserva el más temprano", func(t *testing.T) {
		in := []Sample{
			{T: t0.Add(10 * time.Minute), B2: 2, HasB2: true},
			{T: t0.Add(2 * time.Minute), B2: 1, HasB2: true},
		}
		out := Normalize(in)
		if len(out) != 1 || out[0].B2 != 1 {
			t.Fatalf("out = %+v, se esperaba el más temprano", out)
		}
	})

	t.Run("NaN e Inf se marcan como ausentes", func(t *testing.T) {
		in := []Sample{
			{T: t0, B1: math.NaN(), HasB1: true, B2: math.Inf(1), HasB2: true, Pl: math.Inf(-1), HasPl: true},
		}
		out := Normalize(in)
		if out[0].HasB1 || out[0].HasB2 || out[0].HasPl {
			t.Fatalf("out = %+v, se esperaban valores ausentes", out[0])
		}
	})

	t.Run("NaN en B2 cede el slot a un B2 válido posterior", func(t *testing.T) {
		in := []Sample{
			{T: t0, B2: math.NaN(), HasB2: true},
			{T: t0.Add(3 * time.Minute), B2: 4, HasB2: true},
		}
		out := Normalize(in)
		if len(out) != 1 || out[0].B2 != 4 {
			t.Fatalf("out = %+v, se esperaba B2 = 4", out)
		}
	})
}
