package features

import (
	"math"
	"testing"
)

const eps = 1e-9

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// Fixtures calculados con la fórmula tipo 7 (h = (n−1)·q), idéntica a
// pandas.Series.quantile / numpy.percentile con interpolación lineal.
func TestQuantile(t *testing.T) {
	tests := []struct {
		name string
		xs   []float64
		q    float64
		want float64
	}{
		{"n par", []float64{1, 2, 3, 4}, 0.9, 3.7},
		{"n impar", []float64{1, 2, 3, 4, 5}, 0.9, 4.6},
		{"desordenado con repetidos", []float64{5, 1, 3, 3, 2}, 0.9, 4.2},
		{"n = 1", []float64{7}, 0.9, 7},
		{"mediana por cuantil", []float64{10, 20}, 0.5, 15},
		{"q = 1", []float64{3, 9, 1}, 1, 9},
		{"q = 0", []float64{3, 9, 1}, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Quantile(tt.xs, tt.q); !approx(got, tt.want, eps) {
				t.Fatalf("Quantile = %v, se esperaba %v", got, tt.want)
			}
		})
	}

	if !math.IsNaN(Quantile(nil, 0.9)) {
		t.Fatal("Quantile de vacío debe ser NaN")
	}
}

func TestQuantileNoMutaLaEntrada(t *testing.T) {
	xs := []float64{3, 1, 2}
	Quantile(xs, 0.5)
	if xs[0] != 3 || xs[1] != 1 || xs[2] != 2 {
		t.Fatalf("la entrada fue mutada: %v", xs)
	}
}

func TestMedianAndMAD(t *testing.T) {
	tests := []struct {
		name       string
		xs         []float64
		wantMedian float64
		wantMAD    float64
	}{
		{"n impar", []float64{1, 3, 2}, 2, 1},
		{"n par", []float64{4, 1, 3, 2}, 2.5, 1},
		{"con atípico", []float64{1, 2, 3, 4, 100}, 3, 1},
		{"constante", []float64{5, 5, 5}, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Median(tt.xs); !approx(got, tt.wantMedian, eps) {
				t.Fatalf("Median = %v, se esperaba %v", got, tt.wantMedian)
			}
			if got := MAD(tt.xs); !approx(got, tt.wantMAD, eps) {
				t.Fatalf("MAD = %v, se esperaba %v", got, tt.wantMAD)
			}
		})
	}
}

func TestSpearman(t *testing.T) {
	tests := []struct {
		name         string
		x, y         []float64
		wantRho      float64
		wantConstant bool
	}{
		{"monótona creciente", []float64{1, 2, 3, 4, 5}, []float64{2, 4, 6, 8, 10}, 1, false},
		{"monótona decreciente", []float64{1, 2, 3, 4, 5}, []float64{10, 8, 6, 4, 2}, -1, false},
		// scipy.stats.spearmanr([1,2,2,3],[1,2,3,4]) = 3/√10 (rangos promedio).
		{"empates con rangos promedio", []float64{1, 2, 2, 3}, []float64{1, 2, 3, 4}, 3 / math.Sqrt(10), false},
		{"serie constante", []float64{4, 4, 4}, []float64{1, 2, 3}, 0, true},
		{"menos de 2 pares", []float64{1}, []float64{2}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rho, constant := Spearman(tt.x, tt.y)
			if !approx(rho, tt.wantRho, eps) || constant != tt.wantConstant {
				t.Fatalf("Spearman = (%v, %v), se esperaba (%v, %v)", rho, constant, tt.wantRho, tt.wantConstant)
			}
		})
	}
}
