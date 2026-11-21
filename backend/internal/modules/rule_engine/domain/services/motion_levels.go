package services

import (
	"math"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// Helpers de motion-v2 sobre los niveles horarios N(h) (pitch_now, roll_now
// de las horas CALCULATED), recalculados desde la ventana cruda igual que en
// v1. Reproducen Levels, lmed, last_level y steps de motion_v2_ref.py.

// pyRound redondea como round(x, n) de Python: al decimal más cercano del
// valor binario exacto, con empate al par (strconv hace lo mismo).
func pyRound(v float64, n int) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', n, 64), 64)
	return r
}

func round3(v float64) float64 { return pyRound(v, 3) }

func round4(v float64) float64 { return pyRound(v, 4) }

// axisValue devuelve el eje i (0 = pitch, 1 = roll) del nivel.
func (l *level) axis(i int) float64 {
	if i == 0 {
		return l.p
	}
	return l.r
}

// axesOf traduce el eje de una fila a los ejes que mide.
func axesOf(a string) []struct {
	name string
	i    int
} {
	switch a {
	case entities.MotionAxisPitch:
		return []struct {
			name string
			i    int
		}{{"pitch", 0}}
	case entities.MotionAxisRoll:
		return []struct {
			name string
			i    int
		}{{"roll", 1}}
	}
	return []struct {
		name string
		i    int
	}{{"pitch", 0}, {"roll", 1}}
}

func axisIndex(name string) int {
	if name == "roll" {
		return 1
	}
	return 0
}

// ceilToHour es la primera hora en punto >= x.
func ceilToHour(x time.Time) time.Time {
	h := x.Truncate(time.Hour)
	if h.Before(x) {
		h = h.Add(time.Hour)
	}
	return h
}

// calcBetween devuelve las horas calculadas h con a <= h <= b, ascendentes.
// Antes del inicio de la ventana no hay registros: se recorta ahí.
func (a *assessor) calcBetween(from, to time.Time) []time.Time {
	start := ceilToHour(from)
	if start.Before(a.from) {
		start = ceilToHour(a.from)
	}
	end := to.Truncate(time.Hour)
	var out []time.Time
	for h := start; !h.After(end); h = h.Add(time.Hour) {
		if a.level(h) != nil {
			out = append(out, h)
		}
	}
	return out
}

// lmed es med(a, b): la mediana del eje i de las horas calculadas de [a, b];
// nil si no hay.
func (a *assessor) lmed(from, to time.Time, i int) *float64 {
	hs := a.calcBetween(from, to)
	if len(hs) == 0 {
		return nil
	}
	v := make([]float64, len(hs))
	for k, h := range hs {
		v[k] = a.level(h).axis(i)
	}
	m := median(v)
	return &m
}

// lastLevel es ult(tb): la mediana del eje i de las 4 últimas horas
// calculadas <= tb; nil si no hay.
func (a *assessor) lastLevel(tb time.Time, i int) *float64 {
	var v []float64
	for h := tb.Truncate(time.Hour); !h.Before(a.from) && len(v) < 4; h = h.Add(-time.Hour) {
		if l := a.level(h); l != nil {
			v = append(v, l.axis(i))
		}
	}
	if len(v) == 0 {
		return nil
	}
	m := median(v)
	return &m
}

// stepGroup es un cambio del historial: horas con |N(h) − N(h − 6 h)| >=
// history_step_deg agrupadas si están a <= history_step_group_hours.
type stepGroup struct {
	at, last time.Time
	delta    float64
}

// steps devuelve los cambios del eje i en (from, to], con delta redondeado.
func (a *assessor) steps(from, to time.Time, i int) []entities.MotionStep {
	p := a.p
	w := hours(p.HistoryStepWindowHours)
	var groups []stepGroup
	for _, h := range a.calcBetween(from.Add(time.Second), to) {
		prv := a.level(h.Add(-w))
		if prv == nil {
			continue
		}
		d := a.level(h).axis(i) - prv.axis(i)
		if math.Abs(d) < p.HistoryStepDeg-eps {
			continue
		}
		if n := len(groups); n > 0 && h.Sub(groups[n-1].last) <= hours(p.HistoryStepGroupHours) {
			g := &groups[n-1]
			g.last = h
			if math.Abs(d) > math.Abs(g.delta) {
				g.delta = d
			}
			continue
		}
		groups = append(groups, stepGroup{at: h, last: h, delta: d})
	}
	out := make([]entities.MotionStep, len(groups))
	for k, g := range groups {
		out[k] = entities.MotionStep{At: g.at, Delta: round3(g.delta)}
	}
	return out
}

// stepPattern: ESCALONES si 2 o más cambios explican al menos
// history_step_share del cambio neto; UN_ESCALON si es uno; si no, CONTINUO.
func (a *assessor) stepPattern(st []entities.MotionStep, net *float64) string {
	if net == nil || math.Abs(*net) < eps {
		return entities.MotionPatternContinuo
	}
	tot := 0.0
	for _, s := range st {
		tot += math.Abs(s.Delta)
	}
	if tot >= a.p.HistoryStepShare*math.Abs(*net)-eps {
		switch {
		case len(st) >= 2:
			return entities.MotionPatternEscalones
		case len(st) == 1:
			return entities.MotionPatternUnEscalon
		}
	}
	return entities.MotionPatternContinuo
}

// inEpisode: el cambio cae dentro de un episodio anterior (desde 12 h antes
// de abrir hasta el cierre).
func inEpisode(at time.Time, past []entities.MotionEpisode) bool {
	for _, e := range past {
		if !at.Before(e.OpenedAt.Add(-12*time.Hour)) && !at.After(e.ClearedAt) {
			return true
		}
	}
	return false
}

// axisHistory es el historial del eje i en los days días hasta tb (§6.6).
func (a *assessor) axisHistory(tb time.Time, days, i int, past []entities.MotionEpisode) entities.MotionAxisHistory {
	from := tb.Add(-hours(days * 24))
	out := entities.MotionAxisHistory{StepList: []entities.MotionStep{}}
	now := a.lastLevel(tb, i)
	old := a.lmed(from.Add(-6*time.Hour), from.Add(6*time.Hour), i)
	if now != nil && old != nil {
		out.Net = fp(round3(*now - *old))
		out.RatePerDay = fp(round4(*out.Net / float64(days)))
	}
	if hs := a.calcBetween(from, tb); len(hs) > 0 {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, h := range hs {
			v := a.level(h).axis(i)
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		out.Range = fp(round3(hi - lo))
	}
	nCalc := len(a.calcBetween(from.Add(time.Second), tb))
	out.Coverage = pyRound(float64(nCalc)/float64(days*24), 2)

	st := a.steps(from, tb, i)
	switch {
	case out.Net == nil:
		out.Pattern = entities.MotionPatternSinDatos
	case out.Range != nil && *out.Range <= a.p.StillRangeDeg+eps:
		out.Pattern = entities.MotionPatternQuieto
	default:
		out.Pattern = a.stepPattern(st, out.Net)
	}
	out.Steps = len(st)
	for _, s := range st {
		if inEpisode(s.At, past) {
			out.StepsInAlert++
		}
	}
	if len(st) > 5 {
		st = st[len(st)-5:]
	}
	for _, s := range st {
		in := inEpisode(s.At, past)
		s.InAlert = &in
		out.StepList = append(out.StepList, s)
	}
	return out
}

// history arma el historial de la hora OPENED con "antes" hasta tb.
func (a *assessor) history(tb time.Time, kind entities.MotionEpisodeKind, past []entities.MotionEpisode) *entities.MotionHistory {
	h := &entities.MotionHistory{Until: tb.UTC(), Kind: kind, Normal: []entities.MotionNormalRow{}, PreviousEpisodes: []entities.MotionPreviousEpisode{}}
	for _, d := range entities.MotionHistoryDays {
		ax := entities.MotionAxesHistory{Pitch: a.axisHistory(tb, d, 0, past), Roll: a.axisHistory(tb, d, 1, past)}
		if d == 7 {
			h.D7 = ax
		} else {
			h.D30 = ax
		}
	}
	for _, r := range a.p.Rules {
		if r.WindowHours == 168 || r.WindowHours == 720 {
			h.Normal = append(h.Normal, entities.MotionNormalRow{WindowHours: r.WindowHours, Axis: r.Axis, AmarilloDeg: r.AmarilloDeg})
		}
	}
	limit := tb.Add(-hours(a.p.HistoryPreviousDays * 24))
	for _, e := range past {
		if !e.OpenedAt.Before(limit) {
			h.PreviousEpisodes = append(h.PreviousEpisodes, entities.MotionPreviousEpisode{
				OpenedAt: e.OpenedAt.UTC(), ClearedAt: e.ClearedAt.UTC(), Kind: e.Kind, Level: e.Level,
			})
		}
	}
	if n := len(h.PreviousEpisodes); n > 5 {
		h.PreviousEpisodes = h.PreviousEpisodes[n-5:]
	}
	return h
}

// bestHit es el umbral de mayor nivel y, a igual nivel, de mayor |delta| (el
// primero en empate, como max() de Python); nil si no hay.
func bestHit(hits []entities.MotionRuleHit) *entities.MotionRuleHit {
	var best *entities.MotionRuleHit
	for k := range hits {
		h := &hits[k]
		if best == nil || h.Level.Rank() > best.Level.Rank() ||
			(h.Level == best.Level && math.Abs(h.Delta) > math.Abs(best.Delta)) {
			best = h
		}
	}
	return best
}

// cumulativeReason arma MOV_CUMULATIVE con el umbral largo de mayor nivel.
func (a *assessor) cumulativeReason(t time.Time, long []entities.MotionRuleHit) entities.MotionReason {
	hb := *bestHit(long)
	i := axisIndex(hb.Axis)
	st := a.steps(*hb.Since, t, i)
	r := entities.MotionReason{
		Code: entities.ReasonMovCumulative, Hit: &hb, AmarilloDeg: entities.NormalOf(a.p.Rules, hb.WindowHours, hb.Axis),
		Steps: len(st), Pattern: a.stepPattern(st, &hb.Delta),
	}
	if len(st) > 6 {
		st = st[len(st)-6:]
	}
	r.StepList = st
	if a3, b3 := a.lmed(t.Add(-75*time.Hour), t.Add(-69*time.Hour), i), a.lmed(t.Add(-3*time.Hour), t, i); a3 != nil && b3 != nil {
		r.RatePerDay3d = fp(round4((*b3 - *a3) / 3))
	}
	return r
}

// dailyReason arma MOV_DAILY si t es n × 24 h después de abrir (n >= 1).
func (a *assessor) dailyReason(t time.Time, S entities.MotionState) *entities.MotionReason {
	if !a.p.DailySummary || S.EpisodeOpenedAt == nil {
		return nil
	}
	el := t.Sub(*S.EpisodeOpenedAt)
	if el <= 0 || el%(24*time.Hour) != 0 {
		return nil
	}
	r := &entities.MotionReason{Code: entities.ReasonMovDaily, Day: int(el / (24 * time.Hour))}
	n24 := len(a.calcBetween(t.Add(-23*time.Hour), t))
	if n24 == 0 {
		r.NoData = true
		return r
	}
	r.CalculatedHours24h = n24
	r.Level, r.Sent = S.EpLevel, a.p.SentLevel(S.EpLevel)
	for i, ref := range []*float64{S.EpRefPitch, S.EpRefRoll} {
		b := a.lmed(t.Add(-3*time.Hour), t, i)
		old := a.lmed(t.Add(-27*time.Hour), t.Add(-24*time.Hour), i)
		var d24, tot *float64
		if b != nil && old != nil {
			d24 = fp(round3(*b - *old))
		}
		if b != nil && ref != nil {
			tot = fp(round3(*b - *ref))
		}
		if i == 0 {
			r.DP24h, r.DPTotal = d24, tot
		} else {
			r.DR24h, r.DRTotal = d24, tot
		}
	}
	return r
}
