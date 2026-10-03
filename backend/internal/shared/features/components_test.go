package features

import (
	"math"
	"testing"
	"time"
)

// series crea samples consecutivos cada 15 min con los B2 dados.
func series(b2 ...float64) []Sample {
	out := make([]Sample, len(b2))
	for i, v := range b2 {
		out[i] = Sample{T: t0.Add(time.Duration(i) * Slot), B2: v, HasB2: true}
	}
	return out
}

func constantSeries(n int, v float64) []Sample {
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = v
	}
	return series(vals...)
}

var baseSignal = Signal{MedianB2: 50, ThetaD: 60, MinElevatedB2: 61, HasMinElevated: true, DeltaMin: 1}

func TestComputeComponentsCoverage(t *testing.T) {
	tests := []struct {
		name           string
		n, hours       int
		wantCalculable bool
	}{
		{"16/24 no calculable", 16, 6, false},
		{"17/24 calculable", 17, 6, true},
		{"67/96 no calculable", 67, 24, false},
		{"68/96 calculable", 68, 24, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ComputeComponents(constantSeries(tt.n, 50), tt.hours, baseSignal, false)
			if c.Calculable != tt.wantCalculable {
				t.Fatalf("Calculable = %v (cobertura %.2f), se esperaba %v", c.Calculable, c.Coverage, tt.wantCalculable)
			}
		})
	}

	t.Run("la cobertura tiene tope 100", func(t *testing.T) {
		c := ComputeComponents(constantSeries(30, 50), 6, baseSignal, false)
		if c.Coverage != 100 {
			t.Fatalf("Coverage = %v, se esperaba 100", c.Coverage)
		}
	})

	t.Run("cobertura sobre B2 válido, no sobre recibidos", func(t *testing.T) {
		w := constantSeries(24, 50)
		for i := 0; i < 8; i++ {
			w[i].HasB2 = false
		}
		c := ComputeComponents(w, 6, baseSignal, false)
		if c.NReceived != 24 || c.NValid != 16 || c.Calculable {
			t.Fatalf("NReceived=%d NValid=%d Calculable=%v", c.NReceived, c.NValid, c.Calculable)
		}
	})
}

func TestComputeComponentsDuration(t *testing.T) {
	sig := Signal{MedianB2: 0, ThetaD: 10}
	c := ComputeComponents(series(10, 10, 11), 6, sig, false)

	// B2 == θ_D no es elevado: solo 1 de 3.
	if !approx(c.D, 100.0/3, eps) {
		t.Fatalf("D = %v, se esperaba %v", c.D, 100.0/3)
	}
}

func TestComputeComponentsPersistence(t *testing.T) {
	sig := Signal{MedianB2: 0, ThetaD: 10}

	at := func(offsets ...time.Duration) []Sample {
		out := make([]Sample, len(offsets))
		for i, o := range offsets {
			out[i] = Sample{T: t0.Add(o), B2: 20, HasB2: true}
		}
		return out
	}

	tests := []struct {
		name   string
		window []Sample
		wantP  float64
	}{
		{"racha de 1 sample = 0 h", at(0), 0},
		{"hueco de 20 min no corta", at(0, 20*time.Minute), 20.0 / 60},
		{"hueco de 20 min + 1 s corta", at(0, 20*time.Minute+time.Second), 0},
		{"racha de 5 samples = 1 h", at(0, 15*time.Minute, 30*time.Minute, 45*time.Minute, 60*time.Minute), 1},
		{
			"un no elevado corta la racha",
			append(at(0, 15*time.Minute), Sample{T: t0.Add(30 * time.Minute), B2: 5, HasB2: true}, Sample{T: t0.Add(45 * time.Minute), B2: 20, HasB2: true}),
			0.25,
		},
		{
			"B2 ausente no cuenta como hueco si la separación entre válidos es ≤ 20 min",
			[]Sample{{T: t0, B2: 20, HasB2: true}, {T: t0.Add(5 * time.Minute)}, {T: t0.Add(15 * time.Minute), B2: 20, HasB2: true}},
			0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ComputeComponents(tt.window, 6, sig, false)
			if !approx(c.P, tt.wantP, eps) {
				t.Fatalf("P = %v, se esperaba %v", c.P, tt.wantP)
			}
		})
	}
}

func TestComputeComponentsExcess(t *testing.T) {
	// 12 de 24 samples válidos (con huecos), cada uno 2 por encima de la mediana:
	// Ex = 24 × 24 / 12 = 48.
	w := make([]Sample, 0, 12)
	for i := 0; i < 24; i += 2 {
		w = append(w, Sample{T: t0.Add(time.Duration(i) * Slot), B2: 52, HasB2: true})
	}
	c := ComputeComponents(w, 6, baseSignal, false)
	if !approx(c.Ex, 48, eps) {
		t.Fatalf("Ex = %v, se esperaba 48", c.Ex)
	}

	// Por debajo de la mediana no resta.
	c = ComputeComponents(series(40, 52), 6, baseSignal, false)
	if !approx(c.Ex, 2*24/2.0, eps) {
		t.Fatalf("Ex = %v, se esperaba 24", c.Ex)
	}
}

func coherentWindow(n int, constantPl bool) []Sample {
	out := make([]Sample, n)
	for i := range out {
		pl := float64(i)
		if constantPl {
			pl = 0.5
		}
		out[i] = Sample{
			T:  t0.Add(time.Duration(i) * Slot),
			B1: float64(i), B2: float64(2 * i), Pl: pl,
			HasB1: true, HasB2: true, HasPl: true,
		}
	}
	return out
}

func TestComputeComponentsCoherence(t *testing.T) {
	t.Run("menos de 40 samples completos ⇒ sin C", func(t *testing.T) {
		c := ComputeComponents(coherentWindow(39, false), 24, baseSignal, true)
		if c.HasC {
			t.Fatal("HasC = true, se esperaba false")
		}
	})

	t.Run("40 samples monótonos ⇒ C = 1", func(t *testing.T) {
		c := ComputeComponents(coherentWindow(40, false), 24, baseSignal, true)
		if !c.HasC || !approx(c.C, 1, eps) {
			t.Fatalf("C = %v (HasC %v), se esperaba 1", c.C, c.HasC)
		}
	})

	t.Run("serie constante aporta 0 y se divide por 3", func(t *testing.T) {
		c := ComputeComponents(coherentWindow(40, true), 24, baseSignal, true)
		if !c.HasC || !approx(c.C, 1.0/3, eps) {
			t.Fatalf("C = %v, se esperaba 1/3", c.C)
		}
		if len(c.ConstantSeries) != 1 || c.ConstantSeries[0] != SeriesPlanitud {
			t.Fatalf("ConstantSeries = %v, se esperaba [PLANITUD]", c.ConstantSeries)
		}
	})

	t.Run("withC = false no calcula C", func(t *testing.T) {
		c := ComputeComponents(coherentWindow(40, false), 24, baseSignal, false)
		if c.HasC {
			t.Fatal("HasC = true con withC = false")
		}
	})
}

func TestComputeComponentsPlanitudDuration(t *testing.T) {
	sig := baseSignal
	sig.P90Planitud, sig.HasP90Planitud = 30, true

	c := ComputeComponents(coherentWindow(40, false), 24, sig, false)
	// Pl = 0..39: 9 valores (31..39) > 30.
	if !c.HasDPl || !approx(c.DPl, 100*9/40.0, eps) {
		t.Fatalf("DPl = %v, se esperaba %v", c.DPl, 100*9/40.0)
	}

	c = ComputeComponents(coherentWindow(40, false), 24, baseSignal, false)
	if c.HasDPl {
		t.Fatal("sin P90_Pl no debe haber D_Pl")
	}
}

func TestMADFloor(t *testing.T) {
	tests := []struct {
		name      string
		component string
		hours     int
		sig       Signal
		want      float64
	}{
		{"P", ComponentP, 24, baseSignal, 0.25},
		{"D 24 h", ComponentD, 24, baseSignal, 100.0 / 96},
		{"D 6 h", ComponentD, 6, baseSignal, 100.0 / 24},
		{"C", ComponentC, 24, baseSignal, 0.01},
		{"Ex normal", ComponentEx, 24, baseSignal, 11},
		{"Ex con θ_D = mediana", ComponentEx, 24, Signal{MedianB2: 50, ThetaD: 50, MinElevatedB2: 51, HasMinElevated: true, DeltaMin: 1}, 1},
		{"Ex con diferencia < Δmin", ComponentEx, 24, Signal{MedianB2: 50, ThetaD: 50, MinElevatedB2: 50.5, HasMinElevated: true, DeltaMin: 2}, 2},
		{"Ex degenerada usa Δmin", ComponentEx, 24, Signal{MedianB2: 50, DeltaMin: 1}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MADFloor(tt.component, tt.hours, tt.sig); !approx(got, tt.want, eps) {
				t.Fatalf("MADFloor = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

func TestSigmoidAndIndex(t *testing.T) {
	if !approx(Sigmoid(2), 0.5, eps) {
		t.Fatalf("Sigmoid(2) = %v, se esperaba 0,5", Sigmoid(2))
	}
	if !approx(Sigmoid(4), 1/(1+math.Exp(-1.2)), eps) {
		t.Fatal("Sigmoid(4) incorrecto")
	}
	if !approx(Index(0.5, 0.3, 0.6, 0.9), 0.5*1.8/3, eps) {
		t.Fatal("Index incorrecto")
	}
}

func TestScore(t *testing.T) {
	stats := ComponentStats{
		D:    Stat{Median: 10, MAD: 0, N: 60},  // piso 100/96
		P:    Stat{Median: 1, MAD: 0.1, N: 60}, // piso 0,25
		Ex:   Stat{Median: 100, MAD: 50, N: 60},
		C:    Stat{Median: 0.2, MAD: 0.1, N: 60},
		HasC: true,
	}
	comp := Components{D: 20, P: 3, Ex: 400, C: 0.6, HasC: true}

	sc, ok := Score(comp, stats, 24, baseSignal)
	if !ok {
		t.Fatal("Score no calculable")
	}

	wantZD := (20 - 10) / (100.0 / 96 * MADScale)
	wantZP := (3 - 1) / (0.25 * MADScale)
	wantZEx := (400 - 100) / (50 * MADScale)
	wantZC := (0.6 - 0.2) / (0.1 * MADScale)
	if !approx(sc.ZD, wantZD, eps) || !approx(sc.ZP, wantZP, eps) || !approx(sc.ZEx, wantZEx, eps) || !approx(sc.ZC, wantZC, eps) {
		t.Fatalf("z = %+v", sc)
	}

	wantI := Sigmoid(wantZP) * (Sigmoid(wantZD) + Sigmoid(wantZEx) + Sigmoid(wantZC)) / 3
	if !approx(sc.I, wantI, eps) {
		t.Fatalf("I = %v, se esperaba %v", sc.I, wantI)
	}

	if len(sc.FloorsApplied) != 2 || sc.FloorsApplied[0] != ComponentD || sc.FloorsApplied[1] != ComponentP {
		t.Fatalf("FloorsApplied = %v, se esperaba [D P]", sc.FloorsApplied)
	}

	t.Run("sin C en la ventana no es calculable", func(t *testing.T) {
		c := comp
		c.HasC = false
		if _, ok := Score(c, stats, 24, baseSignal); ok {
			t.Fatal("se esperaba no calculable")
		}
	})

	t.Run("sin estadística de D no es calculable", func(t *testing.T) {
		st := stats
		st.D = Stat{}
		if _, ok := Score(comp, st, 24, baseSignal); ok {
			t.Fatal("se esperaba no calculable")
		}
	})
}
