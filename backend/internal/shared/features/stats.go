package features

import (
	"math"
	"sort"
)

// sortedCopy devuelve una copia ordenada ascendentemente (no muta la entrada).
func sortedCopy(xs []float64) []float64 {
	out := make([]float64, len(xs))
	copy(out, xs)
	sort.Float64s(out)
	return out
}

// Quantile calcula el cuantil q ∈ [0, 1] con interpolación lineal (tipo 7, el
// default de pandas.Series.quantile y numpy.percentile). Vacío ⇒ NaN.
func Quantile(xs []float64, q float64) float64 {
	n := len(xs)
	if n == 0 {
		return math.NaN()
	}

	s := sortedCopy(xs)
	if n == 1 {
		return s[0]
	}

	h := float64(n-1) * q
	lo := int(math.Floor(h))
	if lo >= n-1 {
		return s[n-1]
	}

	return s[lo] + (h-float64(lo))*(s[lo+1]-s[lo])
}

// Median devuelve la mediana (promedio de los dos centrales si n es par,
// igual que numpy.median). Vacío ⇒ NaN.
func Median(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return math.NaN()
	}

	s := sortedCopy(xs)
	if n%2 == 1 {
		return s[n/2]
	}

	return (s[n/2-1] + s[n/2]) / 2
}

// MAD devuelve mediana(|x − mediana(x)|), sin escalar. Vacío ⇒ NaN.
func MAD(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}

	m := Median(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - m)
	}

	return Median(dev)
}

// averageRanks asigna rangos 1..n; los empates reciben el rango promedio.
func averageRanks(xs []float64) []float64 {
	n := len(xs)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })

	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j+1 < n && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			ranks[idx[k]] = avg
		}
		i = j + 1
	}

	return ranks
}

// isConstant indica si todos los valores son iguales (o hay menos de 2).
func isConstant(xs []float64) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i] != xs[0] {
			return false
		}
	}
	return true
}

// Spearman calcula ρ de Spearman (Pearson sobre rangos promedio). Si alguna
// serie es constante (o hay menos de 2 pares) devuelve (0, true).
// x e y deben tener la misma longitud.
func Spearman(x, y []float64) (rho float64, constant bool) {
	if len(x) != len(y) || len(x) < 2 || isConstant(x) || isConstant(y) {
		return 0, true
	}

	rx := averageRanks(x)
	ry := averageRanks(y)

	n := float64(len(rx))
	var mx, my float64
	for i := range rx {
		mx += rx[i]
		my += ry[i]
	}
	mx /= n
	my /= n

	var sxy, sxx, syy float64
	for i := range rx {
		dx := rx[i] - mx
		dy := ry[i] - my
		sxy += dx * dy
		sxx += dx * dx
		syy += dy * dy
	}

	return sxy / math.Sqrt(sxx*syy), false
}
