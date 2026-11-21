package services

import (
	"math"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// tableResult es el resultado del paso 8 de motion-v2 en la hora.
type tableResult struct {
	hits        []entities.MotionRuleHit // los umbrales que cuentan (rules_hit)
	short, long []entities.MotionRuleHit
	tableLevel  entities.Level // máximo nivel de hits (table_level_hour antes del tope)
	hourLevel   entities.Level // nivel de la hora (§6.3)
	closure     bool
	abrupt      bool
}

// tableStep evalúa la tabla de lo normal (§6.2) y el nivel de la hora (§6.3).
// Actualiza rest_at y long_hold del estado.
func (a *assessor) tableStep(S *entities.MotionState, t time.Time, Pn, Rn float64, J, newly, returned bool) tableResult {
	p := a.p
	cur := [2]float64{Pn, Rn}
	if S.RestAt == nil {
		S.RestAt = tp(t)
	}
	var res tableResult
	res.closure = J || S.Displaced || newly || returned ||
		(S.LastJointAt != nil && t.Sub(*S.LastJointAt) <= hours(p.AbruptSuppressAfterJointHours))

	hold := map[string]int{}
	for k, v := range S.LongHold {
		hold[k] = v
	}
	shortNow := false
	for _, th := range entities.MotionThresholds(p.Rules) {
		var hit *entities.MotionRuleHit
		ok := false
		if th.Kind == entities.MotionRuleShort {
			for k := 1; k <= th.WindowHours; k++ {
				lk := a.level(t.Add(-hours(k)))
				if lk == nil {
					continue
				}
				for _, ax := range axesOf(th.Axis) {
					d := cur[ax.i] - lk.axis(ax.i)
					if math.Abs(d) >= th.Deg-eps && (hit == nil || math.Abs(d) > math.Abs(hit.Delta)) {
						hit = &entities.MotionRuleHit{Axis: ax.name, Delta: round3(d), SpanHours: k}
					}
				}
			}
			ok = hit != nil
		} else {
			t0 := t.Add(-hours(th.WindowHours))
			if S.RestAt.After(t0) {
				t0 = *S.RestAt
			}
			// Lo acumulado solo mide movimiento lento: se omite si en la
			// ventana hubo un patrón conjunto o un cambio rápido.
			jointIn := (S.LastPatternAt != nil && !S.LastPatternAt.Before(t0)) ||
				(S.LastFastAt != nil && !S.LastFastAt.Before(t0)) || J || newly || returned || shortNow
			cov := float64(len(a.calcBetween(t0.Add(time.Second), t))) / math.Max(t.Sub(t0).Hours(), 1)
			if t0.Before(t) && !jointIn && cov >= p.LongMinCoverage-eps {
				for _, ax := range axesOf(th.Axis) {
					ref := a.lmed(t0.Add(-3*time.Hour), t0.Add(3*time.Hour), ax.i)
					if ref == nil {
						continue
					}
					d := cur[ax.i] - *ref
					if math.Abs(d) >= th.Deg-eps && (hit == nil || math.Abs(d) > math.Abs(hit.Delta)) {
						hit = &entities.MotionRuleHit{Axis: ax.name, Delta: round3(d), Since: tp(t0), SpanHours: int(t.Sub(t0) / time.Hour)}
					}
				}
				ok = hit != nil
			}
			if ok {
				hold[th.Key]++
			} else {
				delete(hold, th.Key)
			}
			ok = hold[th.Key] >= p.LongHoldHours
		}
		if !ok {
			continue
		}
		hit.WindowHours, hit.Kind, hit.Level, hit.Deg = th.WindowHours, th.Kind, th.Level, th.Deg
		res.hits = append(res.hits, *hit)
		if th.Kind == entities.MotionRuleShort {
			shortNow = true
			res.short = append(res.short, *hit)
		} else {
			res.long = append(res.long, *hit)
		}
	}
	S.LongHold = hold

	res.tableLevel = entities.LevelNormal
	maxShort := 0.0
	for _, h := range res.hits {
		res.tableLevel = entities.MaxLevel(res.tableLevel, h.Level)
	}
	for _, h := range res.short {
		maxShort = math.Max(maxShort, math.Abs(h.Delta))
	}
	switch {
	case res.closure && (res.tableLevel != entities.LevelNormal || J || newly) && maxShort <= p.ClosureMaxDeg+eps:
		res.hourLevel = p.ClosureCapLevel // patrón conjunto: tope
	case J || newly:
		res.hourLevel = entities.MaxLevel(res.tableLevel, entities.LevelAmarillo)
	default:
		res.hourLevel = res.tableLevel
	}
	res.abrupt = len(res.short) > 0 && !res.closure
	if res.hits == nil {
		res.hits = []entities.MotionRuleHit{}
	}
	return res
}

// episodeStep aplica el paso 10 (episodio), el 11 (resumen diario) y el 12
// (historial en la hora OPENED) de motion-v2 (§6.4–§6.6).
func (a *assessor) episodeStep(S *entities.MotionState, ev *entities.MotionEvaluation, t time.Time, tab tableResult,
	J, newly bool, stillRun int, reasons []entities.MotionReason) []entities.MotionReason {
	p := a.p
	has := func(code entities.ReasonCode) bool {
		for _, r := range reasons {
			if r.Code == code {
				return true
			}
		}
		return false
	}
	opener := J || newly || tab.hourLevel != entities.LevelNormal

	if !S.InEpisode && opener {
		S.InEpisode = true
		S.EpisodeOpenedAt = tp(t)
		ev.EpisodeEvent = entities.EpisodeEventOpened

		var kind entities.MotionEpisodeKind
		var tb time.Time
		switch {
		case has(entities.ReasonMovShiftGap):
			kind, tb = entities.MotionEpisodeHueco, *S.GapFrom
		case J || has(entities.ReasonMovShift):
			kind, tb = entities.MotionEpisodeSalto, t.Add(-time.Hour)
			if has(entities.ReasonMovShift) {
				tb = *S.ShiftStartedAt
			}
			tb = tb.Add(-time.Second)
		case len(tab.short) > 0:
			kind, tb = entities.MotionEpisodeRapido, t.Add(-hours(bestHit(tab.short).SpanHours))
		default:
			kind, tb = entities.MotionEpisodeAcumulado, t
		}
		S.EpKind = kind
		S.EpLevel = entities.MaxLevel(tab.hourLevel, entities.LevelAmarillo)
		S.EpHasLong = len(tab.long) > 0
		if kind == entities.MotionEpisodeAcumulado {
			since := *bestHit(tab.long).Since
			S.EpStartAt = tp(since)
			S.EpRefPitch = a.lmed(since.Add(-3*time.Hour), since.Add(3*time.Hour), 0)
			S.EpRefRoll = a.lmed(since.Add(-3*time.Hour), since.Add(3*time.Hour), 1)
		} else {
			S.EpStartAt = tp(tb)
			S.EpRefPitch, S.EpRefRoll = a.lastLevel(tb, 0), a.lastLevel(tb, 1)
		}
		if len(tab.long) > 0 {
			reasons = append(reasons, a.cumulativeReason(t, tab.long))
		}
		ev.History = a.history(tb, kind, a.past)
		return reasons
	}
	if !S.InEpisode {
		return reasons
	}

	if tab.hourLevel.Rank() > S.EpLevel.Rank() {
		var cand []entities.MotionRuleHit
		for _, h := range tab.hits {
			if h.Level == tab.hourLevel {
				cand = append(cand, h)
			}
		}
		r := entities.MotionReason{
			Code: entities.ReasonMovLevelUp, FromLevel: S.EpLevel, ToLevel: tab.hourLevel,
			SentFrom: p.SentLevel(S.EpLevel), SentTo: p.SentLevel(tab.hourLevel),
		}
		if hb := bestHit(cand); hb != nil {
			// MOV_LEVEL_UP no lleva since (§6.4): el motivo guarda lo que viaja.
			h := *hb
			h.Since = nil
			r.Hit = &h
		}
		reasons = append(reasons, r)
		S.EpLevel = tab.hourLevel
	}
	if len(tab.long) > 0 && !S.EpHasLong {
		S.EpHasLong = true
		reasons = append(reasons, a.cumulativeReason(t, tab.long))
	}

	okLine := S.TempState == "" || S.TempState == entities.TempVariable || S.TempState == entities.TempSinEstabilizar
	netOK := true
	if S.EpHasLong {
		for i := 0; i < 2; i++ {
			old := a.lmed(t.Add(-hours(p.LongCloseHours+3)), t.Add(-hours(p.LongCloseHours-3)), i)
			now := a.lmed(t.Add(-3*time.Hour), t, i)
			if old == nil || now == nil || math.Abs(*now-*old) > p.LongCloseNetDeg+eps {
				netOK = false
			}
		}
	}
	if stillRun >= p.StillHours && okLine && len(tab.short) == 0 && netOK && t.Sub(*S.EpisodeOpenedAt) >= hours(p.StillHours) {
		S.InEpisode = false
		ev.EpisodeEvent = entities.EpisodeEventCleared
		var dpt, drt *float64
		if nowp := a.lmed(t.Add(-3*time.Hour), t, 0); nowp != nil && S.EpRefPitch != nil {
			dpt = fp(round3(*nowp - *S.EpRefPitch))
		}
		if nowr := a.lmed(t.Add(-3*time.Hour), t, 1); nowr != nil && S.EpRefRoll != nil {
			drt = fp(round3(*nowr - *S.EpRefRoll))
		}
		moved := (dpt != nil && math.Abs(*dpt) >= p.StillRangeDeg-eps) || (drt != nil && math.Abs(*drt) >= p.StillRangeDeg-eps)
		if S.Displaced || moved {
			S.Displaced = false
			reasons = append(reasons, entities.MotionReason{Code: entities.ReasonMovNewPosition, DPTotal: dpt, DRTotal: drt})
		}
		if S.TempState == entities.TempSinEstabilizar {
			S.TempState = entities.TempVariable
			S.TempSince = tp(t)
		}
		reasons = append(reasons, entities.MotionReason{
			Code: entities.ReasonMovStill, Hours: stillRun, DPTotal: dpt, DRTotal: drt, StartedAt: S.EpStartAt,
			Level: S.EpLevel, Sent: p.SentLevel(S.EpLevel), LongEpisode: S.EpHasLong,
		})
		S.RestAt = tp(t)
		S.LongHold = nil
		return reasons
	}
	if d := a.dailyReason(t, *S); d != nil {
		reasons = append(reasons, *d)
	}
	return reasons
}
