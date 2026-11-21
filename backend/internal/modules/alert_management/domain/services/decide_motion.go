package services

import (
	"math"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

// MotionInput es todo lo que DecideMotion necesita para una hora de movimiento.
type MotionInput struct {
	// State es el estado del flujo MOVIMIENTO del punto.
	State entities.ProcessingState
	// Active es la alerta ACTIVA de MOVIMIENTO del punto, nil si no hay. La de
	// FUGA es otra alerta: nunca se combinan.
	Active  *entities.Alert
	Segment *entities.Segment
	// Last es la última alerta FINALIZADA de MOVIMIENTO del punto (candidata a reactivación).
	Last   *entities.Alert
	Notice entities.MotionNotice
	Policy entities.Policy
	Device entities.Device
}

// DecideMotion (v8, REQ_MOVIMIENTO_UMBRALES_V2 §9) aplica una hora de
// movimiento a la alerta de MOVIMIENTO del punto. Es pura: el caso de uso
// garantiza el orden, filtra duplicados y persiste la Decision igual que la de
// Decide. La alerta abre con OPENED (o reactiva la última) en el nivel enviado
// (computed_level), sube con MOV_LEVEL_UP cuando el nivel enviado sube (si no,
// MOV_LEVEL_OBSERVED), nunca baja y finaliza con CLEARED. MOV_DAILY sale
// también en una hora NO_CALCULABLE. Los mensajes salen en el orden de §9.2.
func DecideMotion(in MotionInput) (Decision, error) {
	n := in.Notice
	if in.Active != nil && (in.Active.MeasuringPointID != n.MeasuringPointID || in.Active.EventType != entities.EventTypeMovement) {
		return Decision{}, ErrNoticeMismatch
	}
	at := n.EvaluatedAt.UTC()
	d := Decision{State: in.State, Outcome: OutcomeNoAlert}
	firstHour := in.State.LastEvaluatedAt == nil
	d.State.MeasuringPointID, d.State.EventType = n.MeasuringPointID, entities.EventTypeMovement
	d.State.LastEvaluatedAt = &at
	m := motionBuilder{builder: builder{in: DecideInput{Policy: in.Policy, Device: in.Device}, at: at}, in: in}

	if !n.Calculated() {
		d.State.LastStatus = entities.StatusNoCalculable
		if in.Active != nil && n.Reason(entities.MotionDaily) != nil {
			a := cloneAlert(*in.Active)
			m.daily(&d, &a)
			d.Alert, d.Outcome = &a, OutcomeUpdated
		}
		return d, m.err
	}
	d.State.LastStatus = entities.StatusCalculated

	opens := n.EpisodeEvent == entities.EpisodeOpened || (firstHour && n.InEpisode && n.EpisodeEvent != entities.EpisodeCleared)
	var a entities.Alert
	switch {
	case in.Active != nil:
		// Un OPENED con la alerta ya activa (p. ej. tras perder el CLEARED) no
		// abre otra: la hora sigue la alerta en curso.
		a = cloneAlert(*in.Active)
		d.Outcome = OutcomeUpdated
	case opens:
		a = m.open(&d)
		m.track(&a)
		d.Alert = &a
		return d, m.err
	default:
		return d, nil
	}

	m.track(&a)
	m.displaced(&d, &a)
	m.levelUp(&d, &a)
	m.temperature(&d, &a, true)
	m.slowOngoing(&d, &a)
	m.daily(&d, &a)
	if n.EpisodeEvent == entities.EpisodeCleared {
		m.cleared(&d, &a)
	}
	d.Alert = &a
	return d, m.err
}

type motionBuilder struct {
	builder
	in MotionInput
}

// track copia el contexto del movimiento a la alerta y actualiza su memoria.
func (m *motionBuilder) track(a *entities.Alert) {
	n := m.in.Notice
	at := m.at
	a.MotionState, a.TempState = n.MotionState, n.TempState
	a.LastMotionEvaluatedAt = &at
	if n.ShiftStartedAt != nil {
		s := n.ShiftStartedAt.UTC()
		a.Runtime.MotionShiftStartedAt = &s
	}
}

// msg arma un mensaje de movimiento con el data del motivo en params["datos"]
// (para poder regenerar el texto).
func (m *motionBuilder) msg(a *entities.Alert, key string, params map[string]string, r *entities.MotionReason,
	kind entities.MessageKind, level entities.Level) entities.Message {
	if r != nil {
		params["datos"] = messages.ReasonDataJSON(r)
	}
	return m.message(a, key, params, kind, level, false)
}

// episodeKind es el tipo del episodio; con un evento v1 se deduce de los motivos.
func episodeKind(n entities.MotionNotice) string {
	if n.EpisodeKind != "" {
		return n.EpisodeKind
	}
	switch {
	case n.Reason(entities.MotionShiftGap) != nil:
		return entities.MotionKindHueco
	case n.Reason(entities.MotionAbrupt) != nil && n.Reason(entities.MotionJump) == nil && n.Reason(entities.MotionShift) == nil:
		return entities.MotionKindRapido
	case n.Reason(entities.MotionCumulative) != nil:
		return entities.MotionKindAcumulado
	}
	return entities.MotionKindSalto
}

// shortHit es el cambio rápido de la hora: MOV_ABRUPT o, si no salió (dentro
// de un patrón conjunto), el umbral corto de mayor nivel y |delta|.
func shortHit(n entities.MotionNotice) *entities.MotionReason {
	if r := n.Reason(entities.MotionAbrupt); r != nil {
		return r
	}
	var best *entities.MotionReason
	for _, h := range n.RulesHit {
		x := entities.MotionReason{Data: h}
		if x.Str("kind") != "SHORT" {
			continue
		}
		if best == nil || entities.Level(x.Str("level")).Rank() > entities.Level(best.Str("level")).Rank() ||
			(x.Str("level") == best.Str("level") && math.Abs(deref(x.Num("delta"))) > math.Abs(deref(best.Num("delta")))) {
			c := x
			c.Data = map[string]any{"axis": h["axis"], "delta": h["delta"], "hours": h["span_hours"],
				"window_hours": h["window_hours"], "level": h["level"], "deg": h["deg"]}
			best = &c
		}
	}
	return best
}

// motionStartedAt (§9.2): RAPIDO, t − (hours − 1) h; HUECO, el fin de la hora
// que contiene gap_from; SALTO y ACUMULADO, t.
func motionStartedAt(n entities.MotionNotice, at time.Time) time.Time {
	switch episodeKind(n) {
	case entities.MotionKindHueco:
		if r := n.Reason(entities.MotionShiftGap); r != nil && r.GapFrom != nil {
			return r.GapFrom.UTC().Truncate(time.Hour).Add(time.Hour)
		}
	case entities.MotionKindRapido:
		if r := shortHit(n); r != nil {
			return at.Add(-time.Duration(max(r.Int("hours"), 1)-1) * time.Hour)
		}
	}
	return at
}

func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// sentLevel es el nivel enviado de la hora (AMARILLO si viene vacío o NORMAL).
func sentLevel(n entities.MotionNotice) entities.Level {
	return entities.MaxLevel(n.ComputedLevel, entities.LevelAmarillo)
}

// tableLevel es el nivel calculado de la hora (el enviado con un evento v1).
func tableLevel(n entities.MotionNotice) entities.Level {
	return entities.MaxLevel(n.TableLevel, sentLevel(n))
}

// openMessage arma MOV_OPEN con su variante según episode_kind (§10.2).
func (m *motionBuilder) openMessage(a *entities.Alert, kind entities.MessageKind, level entities.Level) entities.Message {
	n := m.in.Notice
	t := m.at
	params := map[string]string{
		"calculado": messages.ObservedLevelText(tableLevel(n), sentLevel(n)),
		"historial": "",
	}
	if h := messages.MotionHistoryText(n.History); h != "" {
		params["historial"] = "\n" + h
	}
	var variant string
	var r *entities.MotionReason
	switch episodeKind(n) {
	case entities.MotionKindHueco:
		variant, r = messages.VariantGap, n.Reason(entities.MotionShiftGap)
		if r != nil && r.GapFrom != nil && r.GapTo != nil {
			params["gap_from"], params["gap_to"] = messages.FormatHour(*r.GapFrom), messages.FormatHour(*r.GapTo)
			params["duracion"] = messages.FormatSpan(r.GapTo.Sub(*r.GapFrom).Hours())
		}
		if r != nil {
			params["dp"], params["dr"] = messages.FormatDegreesPtr(r.DP), messages.FormatDegreesPtr(r.DR)
		}
	case entities.MotionKindRapido:
		variant, r = messages.VariantAbrupt, shortHit(n)
		if r != nil {
			hrs := r.Int("hours")
			params["entre"] = messages.Between(t.Add(-time.Duration(hrs)*time.Hour), t)
			params["eje"], params["delta"], params["horas"] = r.Str("axis"), messages.FormatDegreesPtr(r.Num("delta")), messages.FormatWindow(hrs)
			// Un evento v1 no trae la fila: su regla fija era 0,04° en 3 h.
			normal, window := 0.04, 3
			if v := r.Num("amarillo_deg"); v != nil {
				normal = *v
			}
			if w := r.Int("window_hours"); w > 0 {
				window = w
			}
			params["normal"], params["ventana"] = messages.FormatDegreesAbs(normal), messages.FormatWindow(window)
		}
	case entities.MotionKindAcumulado:
		variant, r = messages.VariantCumulative, n.Reason(entities.MotionCumulative)
		if r != nil {
			params["acumulado"] = messages.CumulativeText(t, *r)
		}
	default:
		variant, r = messages.VariantJump, n.Reason(entities.MotionJump)
		params["entre"] = messages.Between(t.Add(-time.Hour), t)
		var dp, dr *float64
		if r != nil {
			dp, dr = r.DP, r.DR
		}
		params["dp"], params["dr"] = messages.FormatDegreesPtr(dp), messages.FormatDegreesPtr(dr)
	}
	return m.msg(a, messages.Key(messages.CodeMovOpen, variant), params, r, kind, level)
}

// open: sin alerta ACTIVA, reactiva la última FINALIZADA (menos de
// merge_window_hours y sin clasificar) o crea una alerta de MOVIMIENTO en el
// nivel enviado. Guarda el historial y el nivel calculado (§9.1).
func (m *motionBuilder) open(d *Decision) entities.Alert {
	n := m.in.Notice
	p := m.in.Policy
	started := motionStartedAt(n, m.at)
	at := m.at
	sent := sentLevel(n)

	last := m.in.Last
	withinWindow := last != nil && last.EndedAt != nil && last.State == entities.AlertFinished &&
		m.at.Sub(*last.EndedAt) < time.Duration(p.MergeWindowHours)*time.Hour
	if withinWindow && (!last.Management.Classified() || !p.ReactivateOnlyIfUnclassified) {
		a := cloneAlert(*last)
		paused := last.Runtime.PausedHours + max(int(started.Sub(*a.EndedAt).Hours()), 0)
		level := entities.MaxLevel(a.CurrentLevel, sent)
		a.State = entities.AlertActive
		a.EndedAt = nil
		a.ReactivationCount++
		a.LastReactivatedAt = &at
		a.CurrentLevel = level
		a.MaxLevel = entities.MaxLevel(a.MaxLevel, level)
		a.BelowLevelHours = 0
		a.Runtime = entities.Runtime{PausedHours: paused}
		a.MotionHistory = n.History
		a.MotionTableLevel = entities.MaxLevel(a.MotionTableLevel, tableLevel(n))
		d.Messages = append(d.Messages, m.openMessage(&a, entities.KindNotification, level))
		d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentReactivation, Level: level, StartedAt: started}
		d.State.LastAlertID = nil
		d.Outcome = OutcomeReactivated
		return a
	}

	a := entities.Alert{
		MeasuringPointID: n.MeasuringPointID,
		EventType:        entities.EventTypeMovement,
		State:            entities.AlertActive,
		StartedAt:        started,
		OpenedAt:         at,
		CurrentLevel:     sent,
		MaxLevel:         sent,
		LastEvaluatedAt:  at,
		HoursCalculated:  1,
		PolicyVersionID:  p.ID,
		Management:       entities.Management{Status: entities.StatusUnclassified},
		MotionHistory:    n.History,
		MotionTableLevel: tableLevel(n),
	}
	d.Messages = append(d.Messages, m.openMessage(&a, "", sent))
	d.IsNew = true
	d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentOpen, Level: sent, StartedAt: started}
	d.Outcome = OutcomeOpened
	return a
}

// shiftStart es el inicio del desplazamiento en curso (o el último conocido).
func shiftStart(a entities.Alert, n entities.MotionNotice, at time.Time) time.Time {
	switch {
	case n.ShiftStartedAt != nil:
		return *n.ShiftStartedAt
	case a.Runtime.MotionShiftStartedAt != nil:
		return *a.Runtime.MotionShiftStartedAt
	case a.TempStableFrom != nil:
		return *a.TempStableFrom
	}
	return at
}

// displaced: un desplazamiento (MOV_SHIFT, MOV_SHIFT_GAP) o cambio rápido
// (MOV_ABRUPT) nuevo en una hora que no abre.
func (m *motionBuilder) displaced(d *Decision, a *entities.Alert) {
	n := m.in.Notice
	for _, code := range []string{entities.MotionShift, entities.MotionShiftGap, entities.MotionAbrupt} {
		r := n.Reason(code)
		if r == nil {
			continue
		}
		params := map[string]string{}
		variant := messages.VariantJump
		switch code {
		case entities.MotionAbrupt:
			variant = messages.VariantAbrupt
			params["entre"] = messages.Between(m.at.Add(-time.Duration(r.Hours)*time.Hour), m.at)
			params["eje"], params["delta"] = r.Axis, messages.FormatDegrees(r.Delta)
		default:
			start := r.StartedAt
			if code == entities.MotionShiftGap {
				variant, start = messages.VariantGap, r.GapFrom
			}
			if start == nil {
				s := shiftStart(*a, n, m.at)
				start = &s
			}
			params["inicio"] = messages.FormatHour(*start)
			params["dp"], params["dr"] = messages.FormatDegreesPtr(r.DP), messages.FormatDegreesPtr(r.DR)
		}
		d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeMovDisplaced, variant), params, r, "", a.CurrentLevel))
	}
}

// levelUp: con MOV_LEVEL_UP, si el nivel enviado sube abre un color nuevo
// (MOV_LEVEL_UP, Notificación); si no, solo registra el nivel calculado
// (MOV_LEVEL_OBSERVED). La alerta nunca baja.
func (m *motionBuilder) levelUp(d *Decision, a *entities.Alert) {
	r := m.in.Notice.Reason(entities.MotionLevelUp)
	if r == nil {
		return
	}
	to := entities.Level(r.Str("to_level"))
	sentFrom, sentTo := entities.Level(r.Str("sent_from")), entities.Level(r.Str("sent_to"))
	a.MotionTableLevel = entities.MaxLevel(a.MotionTableLevel, to)
	params := map[string]string{
		"eje": r.Str("axis"), "delta": messages.FormatDegreesPtr(r.Num("delta")), "span": messages.FormatWindow(r.Int("span_hours")),
		"nivel_calc": messages.LevelName(to), "deg": messages.FormatDegreesAbs(deref(r.Num("deg"))),
		"ventana": messages.FormatWindow(r.Int("window_hours")),
	}
	if sentTo.Rank() > sentFrom.Rank() && sentTo.Rank() > a.CurrentLevel.Rank() {
		at := m.at
		a.CurrentLevel = sentTo
		a.MaxLevel = entities.MaxLevel(a.MaxLevel, sentTo)
		d.CloseSegmentAt = &at
		d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentUp, Level: sentTo, StartedAt: at}
		params["nivel"] = messages.LevelName(sentTo)
		d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeMovLevelUp, ""), params, r, "", sentTo))
		return
	}
	params["nivel_enviado"] = messages.LevelName(a.CurrentLevel)
	d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeMovLevelObserved, ""), params, r, "", a.CurrentLevel))
}

// slowOngoing: MOV_CUMULATIVE en una hora que no abre.
func (m *motionBuilder) slowOngoing(d *Decision, a *entities.Alert) {
	if r := m.in.Notice.Reason(entities.MotionCumulative); r != nil {
		d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeMovSlowOngoing, ""),
			map[string]string{"acumulado": messages.CumulativeText(m.at, *r)}, r, "", a.CurrentLevel))
	}
}

// daily: el resumen de cada 24 h de alerta (MOV_DAILY, §6.5 y §10.2).
func (m *motionBuilder) daily(d *Decision, a *entities.Alert) {
	r := m.in.Notice.Reason(entities.MotionDaily)
	if r == nil {
		return
	}
	params := map[string]string{"n": strconv.Itoa(r.Int("day"))}
	variant := ""
	dp24, dr24, dpt, drt := r.Num("dp_24h"), r.Num("dr_24h"), r.Num("dp_total"), r.Num("dr_total")
	switch {
	case r.Bool("no_data"):
		variant = messages.VariantNoData
	case dp24 == nil || dr24 == nil:
		variant = messages.VariantPartial
		params["n_horas"], params["total"] = strconv.Itoa(r.Int("calculated_hours_24h")), ""
		if dpt != nil {
			params["total"] = " Desde el inicio del movimiento: pitch " + messages.FormatDegreesPtr(dpt) + ", roll " + messages.FormatDegreesPtr(drt) + "."
		}
	default:
		params["dp_dia"], params["dr_dia"] = messages.FormatDegreesPtr(dp24), messages.FormatDegreesPtr(dr24)
		params["dp_total"], params["dr_total"] = messages.FormatDegreesPtr(dpt), messages.FormatDegreesPtr(drt)
		params["sin_cambios"], params["cobertura"] = "", ""
		if math.Abs(*dp24) < 0.01 && math.Abs(*dr24) < 0.01 {
			params["sin_cambios"] = " Sin cambios en las últimas 24 h."
		}
		if h := r.Int("calculated_hours_24h"); h < 20 {
			params["cobertura"] = " Con datos en " + strconv.Itoa(h) + " de las 24 h."
		}
	}
	d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeMovDaily, variant), params, r, "", a.CurrentLevel))
}

// temperature: TEMP_STABILIZED, MOV_RETURN, TEMP_VARIES_AGAIN,
// TEMP_NOT_STABILIZED y TEMP_STILL_STABLE, en ese orden. withReturn = false
// omite MOV_RETURN (es del sensor, no de la temperatura): así se usa en la
// alerta de FUGA vigente (DecideTemperature).
func (m *motionBuilder) temperature(d *Decision, a *entities.Alert, withReturn bool) {
	n := m.in.Notice
	if r := n.Reason(entities.MotionTempStabilized); r != nil {
		from := shiftStart(*a, n, m.at)
		if r.TempStableFrom != nil {
			from = r.TempStableFrom.UTC()
		}
		a.TempStableFrom, a.TempStableTo = &from, nil
		d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeTempStabilized, ""), map[string]string{
			"hora_inicio": messages.FormatClock(from), "fecha_inicio": messages.FormatDate(from),
			"rate": messages.FormatRate(deref(r.TempRateMax6h)),
		}, "", a.CurrentLevel, false))
	}
	if r := n.Reason(entities.MotionReturn); r != nil && withReturn {
		back := m.at.Add(-time.Hour)
		if r.StartedAt != nil {
			back = *r.StartedAt
		}
		d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeMovReturn, ""), map[string]string{
			"hora": messages.FormatClock(back), "dp": messages.FormatDegreesPtr(r.DP), "dr": messages.FormatDegreesPtr(r.DR),
		}, r, "", a.CurrentLevel))
	}
	if r := n.Reason(entities.MotionTempVariesAgain); r != nil {
		at := m.at
		from, to := a.TempStableFrom, r.TempStableTo
		if r.TempStableFrom != nil {
			from = r.TempStableFrom
		}
		if to == nil {
			to = &at
		}
		if from == nil {
			from = to
		}
		a.TempStableFrom, a.TempStableTo = utcPtr(from), utcPtr(to)
		noReturn := ""
		if n.MotionState == entities.MotionDisplaced {
			noReturn = " El sensor no volvió a su posición anterior."
		}
		d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeTempVariesAgain, ""), map[string]string{
			"duracion": messages.FormatDuration(int(math.Round(to.Sub(*from).Hours()))),
			"desde":    messages.FormatHour(*from), "hasta": messages.FormatHour(*to),
			"sin_retorno": noReturn, "rate": messages.FormatRate(r.Rate),
		}, "", a.CurrentLevel, false))
	}
	if r := n.Reason(entities.MotionTempNotStabilized); r != nil {
		d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeTempNotStabilized, ""), map[string]string{
			"horas": strconv.Itoa(r.Hours),
			"dp":    messages.FormatDegrees(deref(n.ShiftPitch)), "dr": messages.FormatDegrees(deref(n.ShiftRoll)),
		}, "", a.CurrentLevel, false))
	}
	if r := n.Reason(entities.MotionTempStillStable); r != nil {
		d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeTempStillStable, ""), map[string]string{
			"horas": strconv.Itoa(r.Hours), "rate": messages.FormatRate(deref(r.TempRateMax6h)),
		}, "", a.CurrentLevel, false))
	}
}

// cleared: el episodio de movimiento terminó; la alerta de MOVIMIENTO
// finaliza con el cambio total y el nivel máximo (END_NORMAL v8).
func (m *motionBuilder) cleared(d *Decision, a *entities.Alert) {
	n := m.in.Notice
	at := m.at
	a.State = entities.AlertFinished
	a.EndedAt = &at
	d.CloseSegmentAt = &at
	r := n.Reason(entities.MotionStill)
	still := entities.MotionReason{}
	if r != nil {
		still = *r
	}
	params := map[string]string{
		"quieto": "24 h sin moverse", "reactivacion": "", "por_magnitud": "", "tramo": "", "nueva_posicion": "",
		"dp_total": messages.FormatDegreesPtr(still.Num("dp_total")), "dr_total": messages.FormatDegreesPtr(still.Num("dr_total")),
		"nivel_enviado": messages.LevelName(a.MaxLevel),
		"duracion":      messages.FormatSpan(at.Sub(a.StartedAt).Hours()),
	}
	if still.Bool("long_episode") {
		params["quieto"] = "3 días sin cambio neto"
	}
	if a.ReactivationCount > 0 && a.LastReactivatedAt != nil {
		params["duracion"] = messages.FormatSpan(at.Sub(*a.LastReactivatedAt).Hours())
		params["reactivacion"] = " desde que se reactivó; la alerta en total, " + messages.FormatSpan(at.Sub(a.StartedAt).Hours())
	}
	if calc := entities.Level(still.Str("max_level")); calc.Rank() > a.MaxLevel.Rank() {
		params["por_magnitud"] = " (por su magnitud correspondía a " + messages.LevelName(calc) + ")"
	}
	if a.TempStableFrom != nil && a.TempStableTo != nil {
		params["tramo"] = " Temperatura estable del " + messages.FormatHour(*a.TempStableFrom) + " al " + messages.FormatHour(*a.TempStableTo) + "."
	}
	if n.Reason(entities.MotionNewPosition) != nil {
		params["nueva_posicion"] = " La posición actual queda como la nueva referencia del sensor."
	}
	d.Messages = append(d.Messages, m.msg(a, messages.Key(messages.CodeEndNormal, messages.VariantMotion), params, r, "", a.CurrentLevel))
	if a.ID != 0 {
		id := a.ID
		d.State.LastAlertID = &id
	}
	d.Outcome = OutcomeFinished
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// TemperatureInput es lo que DecideTemperature necesita: la alerta de FUGA
// ACTIVA del punto y la hora de movimiento que trae motivos de temperatura.
type TemperatureInput struct {
	Alert  entities.Alert
	Notice entities.MotionNotice
	Policy entities.Policy
	Device entities.Device
}

// HasTemperatureReason indica si la hora trae algún mensaje TEMP_*.
func HasTemperatureReason(n entities.MotionNotice) bool {
	for _, code := range []string{entities.MotionTempStabilized, entities.MotionTempVariesAgain,
		entities.MotionTempNotStabilized, entities.MotionTempStillStable} {
		if n.Reason(code) != nil {
			return true
		}
	}
	return false
}

// DecideTemperature replica en la alerta de FUGA vigente los mensajes TEMP_*
// de la hora de movimiento (con su mismo tipo, Notificación o Actualización)
// y su tramo de temperatura estable. Es lo único que el movimiento comparte
// con una alerta de FUGA: no cambia su color, su estado ni su fin, y nunca
// copia el estado del sensor ni emite MOV_*. Sin motivos de temperatura (o si
// la hora no está calculada) no hace nada (Decision.Alert = nil).
func DecideTemperature(in TemperatureInput) (Decision, error) {
	n := in.Notice
	if in.Alert.MeasuringPointID != n.MeasuringPointID || in.Alert.EventType != entities.EventTypeLeak || in.Alert.State != entities.AlertActive {
		return Decision{}, ErrNoticeMismatch
	}
	d := Decision{Outcome: OutcomeNoAlert}
	if !n.Calculated() || !HasTemperatureReason(n) {
		return d, nil
	}
	at := n.EvaluatedAt.UTC()
	m := motionBuilder{builder: builder{in: DecideInput{Policy: in.Policy, Device: in.Device}, at: at}, in: MotionInput{Notice: n}}
	a := cloneAlert(in.Alert)
	if n.TempState != "" {
		a.TempState = n.TempState
	}
	m.temperature(&d, &a, false)
	d.Alert = &a
	d.Outcome = OutcomeUpdated
	return d, m.err
}
