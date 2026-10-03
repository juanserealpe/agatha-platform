package features

import "math"

// Stat es la estadística histórica (mediana y MAD crudo) de un componente.
// N = 0 significa que no hay estadística.
type Stat struct {
	Median, MAD float64
	N           int
}

// ComponentStats agrupa las estadísticas de una longitud de ventana.
type ComponentStats struct {
	D, P, Ex, C Stat
	HasC        bool
}

// Scores son los z, s(z) e I de una ventana.
type Scores struct {
	ZD, ZP, ZEx, ZC float64
	SD, SP, SEx, SC float64
	I               float64
	FloorsApplied   []string // ComponentD | ComponentP | ComponentEx | ComponentC
}

// MADFloor devuelve el piso del MAD crudo del componente:
// P = 0,25 h; D = 100/N_esperados; Ex = máx(MinElevatedB2 − MedianB2, Δmin)
// (Δmin si la señal es degenerada); C = 0,01.
func MADFloor(component string, windowHours int, sig Signal) float64 {
	switch component {
	case ComponentP:
		return PFloorHours
	case ComponentD:
		return 100 / float64(ExpectedSamples(windowHours))
	case ComponentEx:
		if !sig.HasMinElevated {
			return sig.DeltaMin
		}
		return math.Max(sig.MinElevatedB2-sig.MedianB2, sig.DeltaMin)
	case ComponentC:
		return CFloor
	default:
		return 0
	}
}

// Z calcula (x − mediana) / (máx(MAD, piso) × 1,4826) e indica si se aplicó el piso.
func Z(x float64, st Stat, floor float64) (z float64, floorApplied bool) {
	scale := st.MAD
	if scale < floor {
		scale = floor
		floorApplied = true
	}
	return (x - st.Median) / (scale * MADScale), floorApplied
}

// Sigmoid calcula s(z) = 1 / (1 + e^(−0,6 × (z − 2))).
func Sigmoid(z float64) float64 {
	return 1 / (1 + math.Exp(-SigmoidK*(z-SigmoidZ0)))
}

// Index calcula I = s(P) × [s(D) + s(Ex) + s(C)] / 3.
func Index(sP, sD, sEx, sC float64) float64 {
	return sP * (sD + sEx + sC) / 3
}

// Score calcula los scores e I de una ventana contra sus estadísticas
// históricas. Devuelve false si falta C (en la ventana o en la estadística) o
// alguna estadística de D, P o Ex: en ese caso I no es calculable.
func Score(c Components, st ComponentStats, windowHours int, sig Signal) (Scores, bool) {
	if !c.HasC || !st.HasC || st.C.N == 0 || st.D.N == 0 || st.P.N == 0 || st.Ex.N == 0 {
		return Scores{FloorsApplied: []string{}}, false
	}

	sc := Scores{FloorsApplied: []string{}}

	apply := func(component string, x float64, stat Stat) float64 {
		z, floored := Z(x, stat, MADFloor(component, windowHours, sig))
		if floored {
			sc.FloorsApplied = append(sc.FloorsApplied, component)
		}
		return z
	}

	sc.ZD = apply(ComponentD, c.D, st.D)
	sc.ZP = apply(ComponentP, c.P, st.P)
	sc.ZEx = apply(ComponentEx, c.Ex, st.Ex)
	sc.ZC = apply(ComponentC, c.C, st.C)

	sc.SD = Sigmoid(sc.ZD)
	sc.SP = Sigmoid(sc.ZP)
	sc.SEx = Sigmoid(sc.ZEx)
	sc.SC = Sigmoid(sc.ZC)

	sc.I = Index(sc.SP, sc.SD, sc.SEx, sc.SC)

	return sc, true
}
