package services

import (
	"math"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// MotionObservedRow es lo observado para una fila de la tabla de lo normal y
// un eje (GET …/motion-thresholds/observed, §8 de motion-v2, P2).
type MotionObservedRow struct {
	WindowHours int
	Axis        string // "pitch" | "roll"
	RuleAxis    string // ANY | PITCH | ROLL (el de la fila)
	AmarilloDeg float64
	Samples     int
	P95, P99    *float64
	Max         *float64
}

// MotionObserved mide, por cada fila de la tabla y cada eje, el cambio
// observado en esa ventana para las horas calculadas t de [from, to], con la
// definición de §6.2 sin rest_at: una fila corta (≤ 24 h) toma el mayor
// |N(t) − N(t − k h)| con k = 1..W; una larga, |N(t) − N(t − W h)|. Se excluye
// la hora t si alguna hora de [t − W, t] estuvo en episodio (in_episode o la
// hora del CLEARED). Percentiles con interpolación lineal (como numpy);
// grados a 3 decimales. rows debe traer también las horas anteriores a from
// (hasta 720 h) para poder mirar hacia atrás.
func MotionObserved(rows []entities.MotionEvaluation, rules []entities.MotionRule, from, to time.Time) []MotionObservedRow {
	type lv struct{ p, r float64 }
	levels := map[int64]lv{}
	episode := map[int64]bool{}
	var hoursIn []time.Time
	for _, e := range rows {
		t := e.EvaluatedAt.UTC()
		if e.Status == entities.MotionStatusCalculated && e.PitchNow != nil && e.RollNow != nil {
			levels[t.Unix()] = lv{*e.PitchNow, *e.RollNow}
			if !t.Before(from) && !t.After(to) {
				hoursIn = append(hoursIn, t)
			}
		}
		if e.State.InEpisode || e.EpisodeEvent == entities.EpisodeEventCleared {
			episode[t.Unix()] = true
		}
	}
	sort.Slice(hoursIn, func(i, j int) bool { return hoursIn[i].Before(hoursIn[j]) })

	var out []MotionObservedRow
	for _, rule := range rules {
		w := rule.WindowHours
		for _, ax := range axesOf(rule.Axis) {
			var samples []float64
			for _, t := range hoursIn {
				touches := false
				for k := 0; k <= w && !touches; k++ {
					touches = episode[t.Add(-hours(k)).Unix()]
				}
				if touches {
					continue
				}
				cur := levels[t.Unix()]
				val := [2]float64{cur.p, cur.r}[ax.i]
				ks := []int{w}
				if rule.Kind() == entities.MotionRuleShort {
					ks = make([]int, w)
					for k := range ks {
						ks[k] = k + 1
					}
				}
				best, ok := 0.0, false
				for _, k := range ks {
					prev, found := levels[t.Add(-hours(k)).Unix()]
					if !found {
						continue
					}
					d := math.Abs(val - [2]float64{prev.p, prev.r}[ax.i])
					if !ok || d > best {
						best, ok = d, true
					}
				}
				if ok {
					samples = append(samples, best)
				}
			}
			row := MotionObservedRow{WindowHours: w, Axis: ax.name, RuleAxis: rule.Axis, AmarilloDeg: rule.AmarilloDeg, Samples: len(samples)}
			if len(samples) > 0 {
				sort.Float64s(samples)
				row.P95, row.P99 = fp(round3(percentile(samples, 95))), fp(round3(percentile(samples, 99)))
				row.Max = fp(round3(samples[len(samples)-1]))
			}
			out = append(out, row)
		}
	}
	return out
}

// percentile con interpolación lineal sobre valores ordenados (numpy "linear").
func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}
