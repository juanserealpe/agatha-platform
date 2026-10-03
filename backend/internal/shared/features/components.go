package features

import "math"

// Signal es lo que el cálculo de una ventana necesita de la línea base de señal.
type Signal struct {
	MedianB2       float64
	ThetaD         float64
	P90Planitud    float64
	HasP90Planitud bool
	MinElevatedB2  float64
	HasMinElevated bool // false ⇒ SIGNAL_DEGENERATE
	DeltaMin       float64
}

// Components son las variables de una ventana.
type Components struct {
	NExpected, NReceived, NValid int
	Coverage                     float64
	Calculable                   bool // Coverage ≥ CoverageMinPct
	D, P, Ex                     float64
	C                            float64
	HasC                         bool
	DPl                          float64
	HasDPl                       bool
	ConstantSeries               []string // SeriesB1 | SeriesB2 | SeriesPlanitud
}

// ComputeComponents calcula Cobertura, D, P, Ex, C (si withC) y D_Pl de una
// ventana. window debe venir normalizada (ordenada, un sample por slot) y
// contener solo samples de la ventana. Las variables se calculan aunque la
// ventana no sea calculable; el llamador decide con Calculable.
func ComputeComponents(window []Sample, windowHours int, sig Signal, withC bool) Components {
	c := Components{
		NExpected:      ExpectedSamples(windowHours),
		NReceived:      len(window),
		ConstantSeries: []string{},
	}

	for _, s := range window {
		if s.HasB2 {
			c.NValid++
		}
	}

	if c.NExpected > 0 {
		c.Coverage = math.Min(100, 100*float64(c.NValid)/float64(c.NExpected))
	}
	c.Calculable = c.Coverage >= CoverageMinPct

	if c.NValid > 0 {
		c.D, c.P, c.Ex = signalComponents(window, c.NExpected, c.NValid, sig)
	}

	if withC {
		c.C, c.HasC, c.ConstantSeries = coherence(window)
	}

	c.DPl, c.HasDPl = planitudDuration(window, sig)

	return c
}

// signalComponents calcula D, P y Ex sobre los samples con B2 válido.
func signalComponents(window []Sample, nExpected, nValid int, sig Signal) (d, p, ex float64) {
	var (
		elevated    int
		excessSum   float64
		inStreak    bool
		streakStart Sample
		prevValid   Sample
		hasPrev     bool
	)

	for _, s := range window {
		if !s.HasB2 {
			continue
		}

		if excess := s.B2 - sig.MedianB2; excess > 0 {
			excessSum += excess
		}

		isElevated := s.B2 > sig.ThetaD
		if isElevated {
			elevated++
			// Una separación > MaxGap entre samples con B2 válido corta la racha.
			if !inStreak || (hasPrev && s.T.Sub(prevValid.T) > MaxGap) {
				streakStart = s
			}
			inStreak = true
			if hours := s.T.Sub(streakStart.T).Hours(); hours > p {
				p = hours
			}
		} else {
			inStreak = false
		}

		prevValid = s
		hasPrev = true
	}

	d = 100 * float64(elevated) / float64(nValid)
	ex = excessSum * float64(nExpected) / float64(nValid)

	return d, p, ex
}

// coherence calcula C sobre los samples con B1, B2 y Planitud. Requiere al
// menos MinCoherenceSamples; un par con serie constante aporta 0 y siempre se
// divide por 3.
func coherence(window []Sample) (c float64, ok bool, constant []string) {
	constant = []string{}

	var b1, b2, pl []float64
	for _, s := range window {
		if s.HasB1 && s.HasB2 && s.HasPl {
			b1 = append(b1, s.B1)
			b2 = append(b2, s.B2)
			pl = append(pl, s.Pl)
		}
	}

	if len(b1) < MinCoherenceSamples {
		return 0, false, constant
	}

	if isConstant(b1) {
		constant = append(constant, SeriesB1)
	}
	if isConstant(b2) {
		constant = append(constant, SeriesB2)
	}
	if isConstant(pl) {
		constant = append(constant, SeriesPlanitud)
	}

	r12, _ := Spearman(b1, b2)
	r1p, _ := Spearman(b1, pl)
	r2p, _ := Spearman(b2, pl)

	return (math.Abs(r12) + math.Abs(r1p) + math.Abs(r2p)) / 3, true, constant
}

// planitudDuration calcula D_Pl = 100 × N(Pl > P90_Pl) / N(Pl válida). Informativa.
func planitudDuration(window []Sample, sig Signal) (float64, bool) {
	if !sig.HasP90Planitud {
		return 0, false
	}

	var valid, above int
	for _, s := range window {
		if !s.HasPl {
			continue
		}
		valid++
		if s.Pl > sig.P90Planitud {
			above++
		}
	}

	if valid == 0 {
		return 0, false
	}

	return 100 * float64(above) / float64(valid), true
}
