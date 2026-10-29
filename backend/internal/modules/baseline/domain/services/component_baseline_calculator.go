package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// componentsByWindow: C solo en 24 h.
var componentsByWindow = map[int][]string{
	6:  {features.ComponentD, features.ComponentP, features.ComponentEx},
	24: {features.ComponentD, features.ComponentP, features.ComponentEx, features.ComponentC},
}

// ComponentCalculation es el resultado del paso 3 de la sección 9.2.
type ComponentCalculation struct {
	Stats      []entities.ComponentStat // solo combinaciones con NWindows ≥ MinComponentWindows
	Windows6h  int                      // ventanas válidas de 6 h
	Windows24h int                      // ventanas válidas de 24 h
}

// ComponentBaselineCalculator calcula mediana y MAD de cada componente sobre
// las ventanas históricas válidas.
type ComponentBaselineCalculator struct {
	generator  HistoricalWindowGenerator
	minWindows int
}

func NewComponentBaselineCalculator(cfg Config) ComponentBaselineCalculator {
	return ComponentBaselineCalculator{minWindows: cfg.MinComponentWindows}
}

// Calculate descarta las ventanas que solapan la exclusión o no son
// calculables, y usa la señal de esta pasada. samples debe estar normalizado.
func (c ComponentBaselineCalculator) Calculate(
	samples []features.Sample,
	sig features.Signal,
	exclusion entities.ExclusionSet,
	from, to time.Time,
) ComponentCalculation {
	result := ComponentCalculation{Stats: []entities.ComponentStat{}}

	for _, spec := range HistoricalWindows {
		values := map[string][]float64{}
		length := time.Duration(spec.Hours) * time.Hour
		withC := spec.Hours == 24
		valid := 0

		for _, end := range c.generator.Ends(from, to, spec) {
			start := end.Add(-length)
			if exclusion.OverlapsRange(start, end) {
				continue
			}

			comp := features.ComputeComponents(samplesIn(samples, start, end), spec.Hours, sig, withC)
			if !comp.Calculable {
				continue
			}

			valid++
			values[features.ComponentD] = append(values[features.ComponentD], comp.D)
			values[features.ComponentP] = append(values[features.ComponentP], comp.P)
			values[features.ComponentEx] = append(values[features.ComponentEx], comp.Ex)
			if comp.HasC {
				values[features.ComponentC] = append(values[features.ComponentC], comp.C)
			}
		}

		if spec.Hours == 6 {
			result.Windows6h = valid
		} else {
			result.Windows24h = valid
		}

		for _, component := range componentsByWindow[spec.Hours] {
			xs := values[component]
			if len(xs) < c.minWindows {
				continue
			}
			result.Stats = append(result.Stats, entities.ComponentStat{
				WindowHours: spec.Hours,
				Component:   component,
				Median:      features.Median(xs),
				MAD:         features.MAD(xs),
				NWindows:    len(xs),
			})
		}
	}

	return result
}

// HasCompleteStats indica si stats tiene todas las combinaciones de la
// longitud de ventana (24 h: D, P, EX y C; necesarias para calcular I).
func HasCompleteStats(stats []entities.ComponentStat, windowHours int) bool {
	for _, component := range componentsByWindow[windowHours] {
		found := false
		for _, st := range stats {
			if st.WindowHours == windowHours && st.Component == component {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
