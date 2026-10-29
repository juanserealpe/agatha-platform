package features

import (
	"math"
	"reflect"
	"testing"
)

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// Fuente: tabla "Ejemplo con la línea base del estudio (24 h)" del documento de
// flujo de variables derivadas (ventana E1). El estudio publica el denominador
// ya escalado, máx(MAD, piso) × 1,4826; Score recibe el MAD crudo, así que se
// deriva como denominador / 1,4826. P publica 0,371 = 0,25 × 1,4826: su MAD real
// es 0 y el denominador sale del piso (0,25 h).
func e1Inputs() (Components, ComponentStats, Signal) {
	c := Components{
		NExpected: 96, NValid: 96, NReceived: 96,
		Coverage: 100, Calculable: true,
		D: 75.8, P: 3.50, Ex: 24972, C: 0.830, HasC: true,
	}
	st := ComponentStats{
		D:    Stat{Median: 3.33, MAD: 4.42 / MADScale, N: 100},
		P:    Stat{Median: 0, MAD: 0, N: 100},
		Ex:   Stat{Median: 1021, MAD: 1144.6 / MADScale, N: 100},
		C:    Stat{Median: 0.128, MAD: 0.063 / MADScale, N: 100},
		HasC: true,
	}
	// Piso de Ex = máx(97 − 55, 1) = 42: no se activa (MAD crudo ≈ 772).
	sig := Signal{MedianB2: 55, ThetaD: 96, MinElevatedB2: 97, HasMinElevated: true, DeltaMin: 1}
	return c, st, sig
}

func TestScoreGoldenE1(t *testing.T) {
	const (
		zTol = 0.001
		sTol = 0.00001
		iTol = 0.00001
	)

	tests := []struct {
		name   string
		mutate func(c *Components, st *ComponentStats)
		check  func(t *testing.T, sc Scores, ok bool)
	}{
		{
			name: "E1_golden",
			check: func(t *testing.T, sc Scores, ok bool) {
				if !ok {
					t.Fatal("I no calculable")
				}
				zs := []struct {
					name      string
					got, want float64
				}{
					{"zD", sc.ZD, 16.3959}, {"zP", sc.ZP, 9.4429}, {"zEx", sc.ZEx, 20.9252}, {"zC", sc.ZC, 11.1429},
				}
				for _, z := range zs {
					if !approxEqual(z.got, z.want, zTol) {
						t.Errorf("%s = %.6f, want %.4f ± %v", z.name, z.got, z.want, zTol)
					}
				}
				ss := []struct {
					name      string
					got, want float64
				}{
					{"sD", sc.SD, 0.999823}, {"sP", sc.SP, 0.988634}, {"sEx", sc.SEx, 0.999988}, {"sC", sc.SC, 0.995872},
				}
				for _, s := range ss {
					if !approxEqual(s.got, s.want, sTol) {
						t.Errorf("%s = %.8f, want %.6f ± %v", s.name, s.got, s.want, sTol)
					}
				}
				if !approxEqual(sc.I, 0.987211, iTol) {
					t.Errorf("I = %.8f, want 0.987211 ± %v", sc.I, iTol)
				}
				if !approxEqual(sc.I, 0.987, 0.001) {
					t.Errorf("I = %.8f, want 0.987 (publicado) ± 0.001", sc.I)
				}
				if !reflect.DeepEqual(sc.FloorsApplied, []string{ComponentP}) {
					t.Errorf("FloorsApplied = %v, want [P]", sc.FloorsApplied)
				}
			},
		},
		{
			// Piso aplicado después de escalar daría z(P) = 3,5 / 0,25 = 14 e I = 0,997816.
			name: "P_floor_before_scaling",
			check: func(t *testing.T, sc Scores, ok bool) {
				if !ok {
					t.Fatal("I no calculable")
				}
				if !approxEqual(sc.ZP, 9.4429, zTol) {
					t.Errorf("zP = %.6f, want 9.4429", sc.ZP)
				}
				if approxEqual(sc.ZP, 14, zTol) || approxEqual(sc.I, 0.997816, iTol) {
					t.Errorf("piso aplicado después de escalar: zP = %.6f, I = %.8f", sc.ZP, sc.I)
				}
			},
		},
		{
			name:   "P_mad_above_floor",
			mutate: func(_ *Components, st *ComponentStats) { st.P.MAD = 0.5 },
			check: func(t *testing.T, sc Scores, ok bool) {
				if !ok {
					t.Fatal("I no calculable")
				}
				if !approxEqual(sc.ZP, 4.7214, zTol) {
					t.Errorf("zP = %.6f, want 4.7214", sc.ZP)
				}
				if len(sc.FloorsApplied) != 0 {
					t.Errorf("FloorsApplied = %v, want []", sc.FloorsApplied)
				}
			},
		},
		{
			name:   "missing_C",
			mutate: func(c *Components, _ *ComponentStats) { c.HasC = false },
			check: func(t *testing.T, _ Scores, ok bool) {
				if ok {
					t.Fatal("I calculable sin C")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, st, sig := e1Inputs()
			if tt.mutate != nil {
				tt.mutate(&c, &st)
			}
			sc, ok := Score(c, st, 24, sig)
			t.Logf("z = (%.6f, %.6f, %.6f, %.6f) s = (%.8f, %.8f, %.8f, %.8f) I = %.8f floors = %v ok = %v",
				sc.ZD, sc.ZP, sc.ZEx, sc.ZC, sc.SD, sc.SP, sc.SEx, sc.SC, sc.I, sc.FloorsApplied, ok)
			tt.check(t, sc, ok)
		})
	}
}
