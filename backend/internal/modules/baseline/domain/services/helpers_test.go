package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

var t0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func at(hours int) time.Time { return t0.Add(time.Duration(hours) * time.Hour) }

// normalSamples genera una muestra cada 15 min entre from y to con B1, B2 y Planitud
// variando de forma determinista alrededor de B2 ≈ 50 (sin anomalías).
func normalSamples(from, to time.Time) []features.Sample {
	out := []features.Sample{}
	i := 0
	for t := from; t.Before(to); t = t.Add(features.Slot) {
		b2 := 45 + float64((i*7)%11)
		out = append(out, features.Sample{
			T:  t,
			B1: b2*0.8 + float64((i*3)%5), HasB1: true,
			B2: b2, HasB2: true,
			Pl: b2/10 + float64((i*5)%7)*0.1, HasPl: true,
		})
		i++
	}
	return out
}

// withAnomaly sube B2 (y B1/Planitud con él) a ~500 dentro de [from, to).
func withAnomaly(samples []features.Sample, from, to time.Time) []features.Sample {
	out := make([]features.Sample, len(samples))
	copy(out, samples)
	for i := range out {
		if !out[i].T.Before(from) && out[i].T.Before(to) {
			out[i].B2 += 450
			out[i].B1 += 360
			out[i].Pl += 45
		}
	}
	return out
}

func completeStats(n int) []entities.ComponentStat {
	stats := []entities.ComponentStat{}
	for hours, comps := range componentsByWindow {
		for _, c := range comps {
			stats = append(stats, entities.ComponentStat{WindowHours: hours, Component: c, Median: 1, MAD: 1, NWindows: n})
		}
	}
	return stats
}
