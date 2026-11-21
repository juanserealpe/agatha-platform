package services

import (
	"math"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// MotionWindowHours es el largo de la ventana que necesita MotionAssess.
const MotionWindowHours = entities.MotionWindowHours

// eps es la tolerancia de las comparaciones con umbral que la referencia la usa
// (cambio brusco y quieto).
const eps = 1e-9

// MotionAssess evalúa la hora t (que cubre [t − 1 h, t)) con motion-v2 a
// partir de los registros crudos de [t − MotionWindowHours h, t), el estado de
// la hora anterior, los parámetros y los episodios ya cerrados del punto
// (para el historial). Es pura y sin reloj: reproduce motion_v2_ref.py.
//
// Los niveles y saltos de las horas anteriores se recalculan desde la ventana
// en vez de viajar en el estado: así la fila anterior solo lleva lo de §6.1.
func MotionAssess(window []entities.RawRecord, prev entities.MotionState, p entities.MotionParams, t time.Time,
	past []entities.MotionEpisode) entities.MotionEvaluation {
	a := newAssessor(window, p)
	a.from, _ = entities.MotionWindow(t.UTC())
	a.past = past
	return a.assess(prev, t)
}

type level struct {
	p, r float64
	temp *float64
}

type rec struct {
	ts           time.Time
	pitch, roll  float64
	deriva, temp *float64
}

type assessor struct {
	p      entities.MotionParams
	pr     []rec // solo registros con pitch y roll, en orden
	levels map[int64]*level
	lvlOK  map[int64]bool
	jumps  map[int64]bool
	from   time.Time // inicio de la ventana
	past   []entities.MotionEpisode
}

func newAssessor(window []entities.RawRecord, p entities.MotionParams) *assessor {
	pr := make([]rec, 0, len(window))
	for _, w := range window {
		if w.Pitch == nil || w.Roll == nil {
			continue
		}
		pr = append(pr, rec{ts: w.Timestamp.UTC(), pitch: *w.Pitch, roll: *w.Roll, deriva: w.Deriva, temp: w.Temp})
	}
	sort.SliceStable(pr, func(i, j int) bool { return pr[i].ts.Before(pr[j].ts) })
	return &assessor{p: p, pr: pr, levels: map[int64]*level{}, lvlOK: map[int64]bool{}, jumps: map[int64]bool{}}
}

// closedLeft devuelve los registros de [a, b).
func (a *assessor) closedLeft(from, to time.Time) []rec {
	i := sort.Search(len(a.pr), func(k int) bool { return !a.pr[k].ts.Before(from) })
	j := sort.Search(len(a.pr), func(k int) bool { return !a.pr[k].ts.Before(to) })
	if j < i {
		j = i
	}
	return a.pr[i:j]
}

// closedRight devuelve los registros de (a, b].
func (a *assessor) closedRight(from, to time.Time) []rec {
	i := sort.Search(len(a.pr), func(k int) bool { return a.pr[k].ts.After(from) })
	j := sort.Search(len(a.pr), func(k int) bool { return a.pr[k].ts.After(to) })
	if j < i {
		j = i
	}
	return a.pr[i:j]
}

func median(vals []float64) float64 {
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func pitches(rs []rec) []float64 {
	out := make([]float64, len(rs))
	for i, r := range rs {
		out[i] = r.pitch
	}
	return out
}

func rolls(rs []rec) []float64 {
	out := make([]float64, len(rs))
	for i, r := range rs {
		out[i] = r.roll
	}
	return out
}

// medians devuelve las medianas de pitch y roll y la cantidad de registros.
func medians(rs []rec) (float64, float64, int) {
	if len(rs) == 0 {
		return 0, 0, 0
	}
	return median(pitches(rs)), median(rolls(rs)), len(rs)
}

// level es el nivel de la hora h (mediana de [h − 1 h, h)); nil si NO_CALCULABLE.
func (a *assessor) level(h time.Time) *level {
	k := h.Unix()
	if ok, seen := a.lvlOK[k]; seen {
		if !ok {
			return nil
		}
		return a.levels[k]
	}
	w := a.closedLeft(h.Add(-time.Hour), h)
	if len(w) < a.p.MinRecordsHour {
		a.lvlOK[k] = false
		return nil
	}
	pm, rm, _ := medians(w)
	var temps []float64
	for _, r := range w {
		if r.temp != nil {
			temps = append(temps, *r.temp)
		}
	}
	l := &level{p: pm, r: rm}
	if len(temps) > 0 {
		m := median(temps)
		l.temp = &m
	}
	a.levels[k] = l
	a.lvlOK[k] = true
	return l
}

// jumpVars son las variables del salto de la hora h (ya calculada).
type jumpVars struct {
	dp1, dr1 *float64
	der      *float64
	jump     bool
}

func (a *assessor) jumpVarsAt(h time.Time, l *level) jumpVars {
	var v jumpVars
	bp, br, nb := medians(a.closedLeft(h.Add(-3*time.Hour), h.Add(-time.Hour)))
	if nb >= 4 {
		dp, dr := l.p-bp, l.r-br
		v.dp1, v.dr1 = &dp, &dr
	}
	for _, r := range a.closedLeft(h.Add(-time.Hour), h) {
		if r.deriva != nil && (v.der == nil || *r.deriva > *v.der) {
			d := *r.deriva
			v.der = &d
		}
	}
	v.jump = (v.der != nil && *v.der >= a.p.DerivaJumpDeg) ||
		(v.dp1 != nil && v.dr1 != nil && math.Abs(*v.dp1) >= a.p.JumpDeg && math.Abs(*v.dr1) >= a.p.JumpDeg &&
			sign(*v.dp1) == sign(*v.dr1))
	return v
}

// jumpAt dice si la hora h tuvo salto (false si NO_CALCULABLE).
func (a *assessor) jumpAt(h time.Time) bool {
	k := h.Unix()
	if j, ok := a.jumps[k]; ok {
		return j
	}
	l := a.level(h)
	j := l != nil && a.jumpVarsAt(h, l).jump
	a.jumps[k] = j
	return j
}

// anyJumpSince dice si alguna hora de [t − n h, t] (incluida t) tuvo salto.
func (a *assessor) anyJumpSince(t time.Time, n int) bool {
	for k := n; k >= 0; k-- {
		if a.jumpAt(t.Add(-hours(k))) {
			return true
		}
	}
	return false
}

// rate es |temp_now(h) − temp_now(h − 1 h)|; nil si falta alguna.
func (a *assessor) rate(h time.Time) *float64 {
	x, y := a.level(h), a.level(h.Add(-time.Hour))
	if x == nil || y == nil || x.temp == nil || y.temp == nil {
		return nil
	}
	v := math.Abs(*x.temp - *y.temp)
	return &v
}

// gapsAt devuelve los huecos que se evalúan en la hora t: pares consecutivos
// prev, f con f − prev ≥ gap_min_hours y floor(f + gap_post) + 1 h = t.
func (a *assessor) gapsAt(t time.Time) []entities.PendingGap {
	minGap := hours(a.p.GapMinHours)
	post := time.Duration(a.p.GapPostMinutes) * time.Minute
	var out []entities.PendingGap
	for i := 1; i < len(a.pr); i++ {
		prev, f := a.pr[i-1].ts, a.pr[i].ts
		if f.Sub(prev) < minGap {
			continue
		}
		if f.Add(post).Truncate(time.Hour).Add(time.Hour).Equal(t) {
			out = append(out, entities.PendingGap{From: prev, To: f})
		}
	}
	return out
}

func sign(v float64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func fp(v float64) *float64 { return &v }

func tp(v time.Time) *time.Time { return &v }

func hours(n int) time.Duration { return time.Duration(n) * time.Hour }

func cloneState(s entities.MotionState) entities.MotionState {
	s.PendingGaps = append([]entities.PendingGap(nil), s.PendingGaps...)
	if s.LongHold != nil {
		h := make(map[string]int, len(s.LongHold))
		for k, v := range s.LongHold {
			h[k] = v
		}
		s.LongHold = h
	}
	return s
}

func (a *assessor) assess(prev entities.MotionState, t time.Time) entities.MotionEvaluation {
	p := a.p
	t = t.UTC()
	S := cloneState(prev)
	if p.TempMode != entities.TempModeMotionAndTemp {
		S.TempState = ""
	} else if S.TempState == "" {
		S.TempState = entities.TempVariable
	}

	w := a.closedLeft(t.Add(-time.Hour), t)
	n := len(w)
	ev := entities.MotionEvaluation{
		EvaluatedAt:      t,
		NRecords:         n,
		Coverage:         float64(n) / float64(p.RecordsPerHour),
		EpisodeEvent:     entities.EpisodeEventNone,
		AlgorithmVersion: entities.MotionAlgorithmVersion,
	}
	if n > 0 {
		ev.InputMaxTS = tp(w[n-1].ts)
	}
	gaps := a.gapsAt(t)

	// 1) Cobertura.
	if n < p.MinRecordsHour {
		ev.Status = entities.MotionStatusNoCalculable
		S.PendingGaps = append(S.PendingGaps, gaps...)
		if S.Displaced {
			ev.MotionState = entities.MotionDesplazado
		}
		// 11) El resumen diario sale también en una hora NO_CALCULABLE.
		if S.InEpisode {
			if d := a.dailyReason(t, S); d != nil {
				ev.Reasons = append(ev.Reasons, *d)
			}
		}
		return a.finish(ev, S)
	}
	ev.Status = entities.MotionStatusCalculated

	// 2) Variables.
	lv := a.level(t)
	Pn, Rn := lv.p, lv.r
	ev.PitchNow, ev.RollNow, ev.TempNow = fp(Pn), fp(Rn), lv.temp

	jv := a.jumpVarsAt(t, lv)
	J := jv.jump
	a.jumps[t.Unix()] = J
	ev.DP1h, ev.DR1h, ev.DerivaMax1h, ev.Jump = jv.dp1, jv.dr1, jv.der, J

	Bp, Br, nB := medians(a.closedLeft(t.Add(-hours(p.BaseFromHours)), t.Add(-hours(p.BaseToHours))))
	okB := nB >= p.BaseMinRecords
	if okB {
		ev.BasePitch, ev.BaseRoll = fp(Bp), fp(Br)
		ev.DP6h, ev.DR6h = fp(Pn-Bp), fp(Rn-Br)
	}

	w2 := a.closedLeft(t.Add(-hours(p.ShiftWindowHours)), t)
	jsign := 0
	if okB && len(w2) >= p.ShiftMinRecords {
		up, dn := true, true
		for _, r := range w2 {
			if !(r.pitch-Bp >= p.ShiftDeg && r.roll-Br >= p.ShiftDeg) {
				up = false
			}
			if !(r.pitch-Bp <= -p.ShiftDeg && r.roll-Br <= -p.ShiftDeg) {
				dn = false
			}
		}
		if up {
			jsign = 1
		} else if dn {
			jsign = -1
		}
	}
	ev.ShiftRecords2h, ev.ShiftSignNow = len(w2), jsign

	var dp24, dr24 *float64
	if l24 := a.level(t.Add(-24 * time.Hour)); l24 != nil {
		dp24, dr24 = fp(Pn-l24.p), fp(Rn-l24.r)
	}
	ev.DP24h, ev.DR24h = dp24, dr24
	if l7 := a.level(t.Add(-168 * time.Hour)); l7 != nil {
		ev.TrendFlag = math.Abs(Pn-l7.p) >= p.TrendDeg7d || math.Abs(Rn-l7.r) >= p.TrendDeg7d
	}

	var r6 []float64
	for k := 0; k < p.TempFlatHours; k++ {
		if r := a.rate(t.Add(-hours(k))); r != nil {
			r6 = append(r6, *r)
		}
	}
	var maxR6 *float64
	for _, v := range r6 {
		if maxR6 == nil || v > *maxR6 {
			maxR6 = fp(v)
		}
	}
	flat := len(r6) >= p.TempFlatMinRates && maxR6 != nil && *maxR6 < p.TempFlatRate
	rl := make([]*float64, p.TempFlowHours)
	flow := true
	for k := range rl {
		rl[k] = a.rate(t.Add(-hours(k)))
		if rl[k] == nil || *rl[k] < p.TempFlowRate {
			flow = false
		}
	}
	ev.TempRateMax6h, ev.TempFlat, ev.TempFlow = maxR6, flat, flow
	if len(rl) > 0 {
		ev.TempRate = rl[0]
	}

	var reasons []entities.MotionReason
	newly := false

	// 3–4) Hueco (los pendientes primero).
	todo := append(S.PendingGaps, gaps...)
	S.PendingGaps = nil
	preDur := hours(p.GapPreHours)
	postDur := time.Duration(p.GapPostMinutes) * time.Minute
	for _, g := range todo {
		preP, preR, npre := medians(a.closedRight(g.From.Add(-preDur), g.From))
		postP, postR, npost := medians(a.closedRight(g.To.Add(-time.Second), g.To.Add(postDur)))
		if npre < p.GapPreMinRecords || npost < p.GapPostMinRecords {
			continue
		}
		dpg, drg := postP-preP, postR-preR
		gsign := 0
		if dpg >= p.ShiftDeg && drg >= p.ShiftDeg {
			gsign = 1
		} else if dpg <= -p.ShiftDeg && drg <= -p.ShiftDeg {
			gsign = -1
		}
		ev.EvalGapFrom, ev.EvalGapTo, ev.DPGap, ev.DRGap = tp(g.From), tp(g.To), fp(dpg), fp(drg)
		if gsign != 0 && !S.Displaced {
			S.Displaced = true
			S.RefPitch, S.RefRoll = fp(preP), fp(preR)
			S.ShiftSign = gsign
			S.ShiftStartedAt = tp(g.From)
			S.ShiftLevelPitch, S.ShiftLevelRoll = fp(postP), fp(postR)
			S.GapFrom, S.GapTo = tp(g.From), tp(g.To)
			S.LastJointAt = tp(t)
			reasons = append(reasons, entities.MotionReason{
				Code: entities.ReasonMovShiftGap, DP: fp(round3(dpg)), DR: fp(round3(drg)),
				GapFrom: tp(g.From), GapTo: tp(g.To),
			})
			newly = true
		}
		// Un hueco en sentido contrario estando desplazado no hace nada: el
		// retorno lo decide la tolerancia.
	}

	// 5) Desplazamiento sostenido.
	supRet := S.LastReturnAt != nil && t.Sub(*S.LastReturnAt) < hours(p.ShiftSuppressAfterReturnHours)
	// Solo cuenta si fue brusco: al menos una hora con salto en [t − 6 h, t].
	// Un desplazamiento lento en los dos ejes es normal y no abre nada.
	if !S.Displaced && jsign != 0 && !supRet && a.anyJumpSince(t, 6) {
		st := a.shiftStart(t, jsign, Bp, Br, w)
		S.Displaced = true
		S.RefPitch, S.RefRoll = fp(Bp), fp(Br)
		S.ShiftSign = jsign
		S.ShiftStartedAt = tp(st)
		S.ShiftLevelPitch, S.ShiftLevelRoll = fp(Pn), fp(Rn)
		S.GapFrom, S.GapTo = nil, nil
		S.LastJointAt = tp(t)
		reasons = append(reasons, entities.MotionReason{
			Code: entities.ReasonMovShift, DP: fp(round3(Pn - Bp)), DR: fp(round3(Rn - Br)), StartedAt: tp(st),
		})
		newly = true
	}

	// 6) Retorno.
	returned := false
	if S.Displaced && !newly {
		sg := float64(S.ShiftSign)
		slp, slr := *S.ShiftLevelPitch, *S.ShiftLevelRoll
		recp, recr := sg*(slp-Pn), sg*(slr-Rn)
		f := p.ReturnFraction
		if recp >= f*sg*(slp-*S.RefPitch) && recr >= f*sg*(slr-*S.RefRoll) {
			rs := t.Add(-time.Hour)
			for _, r := range a.closedLeft(t.Add(-6*time.Hour), t) {
				if sg*(slp-r.pitch) >= p.JumpDeg && sg*(slr-r.roll) >= p.JumpDeg {
					rs = r.ts
					break
				}
			}
			S.Displaced = false
			S.ReturnStartedAt = tp(rs)
			S.LastJointAt, S.LastReturnAt = tp(t), tp(t)
			reasons = append(reasons, entities.MotionReason{
				Code: entities.ReasonMovReturn, DP: fp(round3(Pn - slp)), DR: fp(round3(Rn - slr)), StartedAt: tp(rs),
			})
			returned = true
		}
	}

	// 3) Salto (el motivo va después del desplazamiento y del retorno).
	if J {
		r := entities.MotionReason{Code: entities.ReasonMovJump, Deriva: jv.der}
		if jv.dp1 != nil {
			r.DP = fp(round3(*jv.dp1))
		}
		if jv.dr1 != nil {
			r.DR = fp(round3(*jv.dr1))
		}
		reasons = append(reasons, r)
	}

	// 8) Tabla de lo normal (motion-v2, §6.2).
	tab := a.tableStep(&S, t, Pn, Rn, J, newly, returned)
	ev.RulesHit, ev.TableLevelHour = tab.hits, tab.hourLevel
	ev.Abrupt = tab.abrupt
	if tab.abrupt && !S.AbruptActive {
		hb := *bestHit(tab.short)
		reasons = append(reasons, entities.MotionReason{
			Code: entities.ReasonMovAbrupt, Hit: &hb, AmarilloDeg: entities.NormalOf(p.Rules, hb.WindowHours, hb.Axis),
		})
	}
	S.AbruptActive = tab.abrupt
	ev.SlowDriftFlag = dp24 != nil && dr24 != nil &&
		(math.Abs(*dp24) >= p.SlowDriftDeg24h || math.Abs(*dr24) >= p.SlowDriftDeg24h)

	// 9) Quieto.
	still := false
	var pp, rr []float64
	anyJump := false
	for k := p.StillHours - 1; k >= 0; k-- {
		h := t.Add(-hours(k))
		l := a.level(h)
		if l == nil {
			continue
		}
		pp, rr = append(pp, l.p), append(rr, l.r)
		if a.jumpAt(h) {
			anyJump = true
		}
	}
	if len(pp) >= p.StillMinCalculated && !anyJump {
		still = spread(pp) <= p.StillRangeDeg+eps && spread(rr) <= p.StillRangeDeg+eps
	}
	stillRun := 0
	if still {
		stillRun = p.StillHours
	}
	ev.Still = still

	// 7) Temperatura.
	if S.TempState != "" {
		reasons = a.tempStep(&S, t, newly, returned, flat, flow, maxR6, rl, reasons)
	}

	// 10–12) Episodio, resumen diario e historial.
	if J || newly || returned {
		S.LastPatternAt = tp(t)
	}
	if len(tab.short) > 0 {
		S.LastFastAt = tp(t)
	}
	reasons = a.episodeStep(&S, &ev, t, tab, J, newly, stillRun, reasons)

	ev.Reasons = reasons
	switch {
	case S.Displaced:
		ev.MotionState = entities.MotionDesplazado
	case ev.HasReason(entities.ReasonMovReturn):
		ev.MotionState = entities.MotionRetorno
	case ev.HasReason(entities.ReasonMovJump):
		ev.MotionState = entities.MotionSalto
	case S.AbruptActive:
		ev.MotionState = entities.MotionBrusco
	default:
		ev.MotionState = entities.MotionNormal
	}
	return a.finish(ev, S)
}

// finish arma la salida (§6.4) y deja en cero lo del episodio fuera de él,
// para que el estado en memoria sea idéntico al que vuelve de la fila.
func (a *assessor) finish(ev entities.MotionEvaluation, S entities.MotionState) entities.MotionEvaluation {
	ev.ComputedLevel, ev.TableLevel = entities.LevelNormal, entities.LevelNormal
	if S.InEpisode {
		ev.TableLevel = S.EpLevel
		ev.ComputedLevel = a.p.SentLevel(S.EpLevel)
		ev.EpisodeKind = S.EpKind
	} else {
		if ev.EpisodeEvent == entities.EpisodeEventCleared {
			ev.EpisodeKind = S.EpKind
		} else {
			S.EpKind = ""
		}
		S.EpLevel, S.EpStartAt, S.EpRefPitch, S.EpRefRoll, S.EpHasLong = "", nil, nil, nil, false
	}
	if len(S.LongHold) == 0 {
		S.LongHold = nil
	}
	ev.State = S
	return ev
}

// shiftStart es el inicio de un desplazamiento sostenido: el primer registro de
// [t − 6 h, t) con deriva ≥ deriva_jump; si no, el primero de la primera hora
// con salto en [t − 6 h, t]; si no, el primero de [t − 6 h, t) con los dos ejes
// a jump_deg o más de la base en el sentido del signo; si no, el primero de la hora.
func (a *assessor) shiftStart(t time.Time, jsign int, Bp, Br float64, w []rec) time.Time {
	p := a.p
	w6 := a.closedLeft(t.Add(-6*time.Hour), t)
	for _, r := range w6 {
		if r.deriva != nil && *r.deriva >= p.DerivaJumpDeg {
			return r.ts
		}
	}
	for k := 6; k >= 0; k-- {
		h := t.Add(-hours(k))
		if a.jumpAt(h) {
			if rs := a.closedLeft(h.Add(-time.Hour), h); len(rs) > 0 {
				return rs[0].ts
			}
		}
	}
	sg := float64(jsign)
	for _, r := range w6 {
		if sg*(r.pitch-Bp) >= p.JumpDeg && sg*(r.roll-Br) >= p.JumpDeg {
			return r.ts
		}
	}
	return w[0].ts
}

// tempStep aplica las transiciones de la temperatura (§5.6) en el orden de la referencia.
func (a *assessor) tempStep(S *entities.MotionState, t time.Time, newly, returned, flat, flow bool,
	maxR6 *float64, rl []*float64, reasons []entities.MotionReason) []entities.MotionReason {
	p := a.p
	ls := S.TempState
	switch {
	case ls == entities.TempVariable && newly:
		ls = entities.TempVerificando
		reasons = append(reasons, entities.MotionReason{Code: entities.ReasonTempVerifying})
	case ls == entities.TempVerificando:
		if returned {
			ls = entities.TempVariable
		} else if flat {
			ls = entities.TempEstable
			S.TempStableFrom = S.ShiftStartedAt
			S.TempStableTo = nil
			reasons = append(reasons, entities.MotionReason{
				Code: entities.ReasonTempStabilized, TempRateMax6h: fp(round3(*maxR6)), TempStableFrom: S.ShiftStartedAt,
			})
		} else if S.ShiftStartedAt != nil && t.Sub(*S.ShiftStartedAt) >= hours(p.StabilizeMaxHours) {
			ls = entities.TempSinEstabilizar
			reasons = append(reasons, entities.MotionReason{Code: entities.ReasonTempNotStabilized, Hours: p.StabilizeMaxHours})
		}
	case ls == entities.TempSinEstabilizar:
		if returned {
			ls = entities.TempVariable
		}
	}
	if ls == entities.TempEstable && returned {
		ls = entities.TempRetornoEstable
		S.StillStableMsgs = 0
		S.TempSince = tp(t)
	}
	if ls == entities.TempEstable || ls == entities.TempRetornoEstable {
		if flow {
			end := t.Add(-hours(p.TempFlowHours))
			if ls == entities.TempRetornoEstable && S.ReturnStartedAt != nil {
				end = *S.ReturnStartedAt
			}
			S.TempStableTo = tp(end)
			reasons = append(reasons, entities.MotionReason{
				Code: entities.ReasonTempVariesAgain, Rate: round3(*rl[0]), TempStableFrom: S.TempStableFrom, TempStableTo: tp(end),
			})
			ls = entities.TempVariable
		} else if ls == entities.TempRetornoEstable && S.TempSince != nil {
			hrs := t.Sub(*S.TempSince).Hours()
			due := float64(p.StillStableFirstHours + S.StillStableMsgs*p.StillStableEveryHours)
			if hrs >= due {
				r := entities.MotionReason{Code: entities.ReasonTempStillStable, Hours: int(hrs)}
				if maxR6 != nil {
					r.TempRateMax6h = fp(round3(*maxR6))
				}
				reasons = append(reasons, r)
				S.StillStableMsgs++
			}
		}
	}
	if ls != S.TempState {
		S.TempSince = tp(t)
	}
	S.TempState = ls
	return reasons
}

func spread(v []float64) float64 {
	lo, hi := v[0], v[0]
	for _, x := range v[1:] {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	return hi - lo
}
