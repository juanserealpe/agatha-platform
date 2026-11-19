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
	// Active es la alerta ACTIVA del punto (de cualquier origen), nil si no hay.
	Active  *entities.Alert
	Segment *entities.Segment
	// Last es la última alerta FINALIZADA del punto (candidata a reactivación).
	Last   *entities.Alert
	Notice entities.MotionNotice
	// LeakInEpisode es el in_episode de FUGA en su última hora procesada.
	LeakInEpisode bool
	Policy        entities.Policy
	Device        entities.Device
}

// DecideMotion aplica una hora de movimiento (§7.4). Es pura: el caso de uso
// garantiza el orden, filtra duplicados y persiste la Decision igual que la de
// Decide. Los mensajes salen en el orden de la tabla de §7.4.
func DecideMotion(in MotionInput) (Decision, error) {
	n := in.Notice
	if in.Active != nil && in.Active.MeasuringPointID != n.MeasuringPointID {
		return Decision{}, ErrNoticeMismatch
	}
	at := n.EvaluatedAt.UTC()
	d := Decision{State: in.State, Outcome: OutcomeNoAlert}
	firstHour := in.State.LastEvaluatedAt == nil
	d.State.MeasuringPointID, d.State.EventType = n.MeasuringPointID, entities.EventTypeMovement
	d.State.LastEvaluatedAt = &at
	d.State.InEpisode = n.InEpisode

	if !n.Calculated() {
		d.State.LastStatus = entities.StatusNoCalculable
		return d, nil
	}
	d.State.LastStatus = entities.StatusCalculated

	m := motionBuilder{builder: builder{in: DecideInput{Policy: in.Policy, Device: in.Device}, at: at}, in: in}
	opens := n.EpisodeEvent == entities.EpisodeOpened || (firstHour && n.InEpisode && n.EpisodeEvent != entities.EpisodeCleared)

	var a entities.Alert
	switch {
	case opens && in.Active == nil:
		a = m.open(&d)
	case opens:
		a = cloneAlert(*in.Active)
		m.joined(&d, &a)
	case in.Active != nil:
		a = cloneAlert(*in.Active)
		d.Outcome = OutcomeUpdated
	default:
		return d, nil
	}

	m.track(&a)
	if !opens {
		m.displaced(&d, &a)
	}
	m.temperatureAndReturn(&d, &a)
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

// openVariant es la variante de MOV_OPEN: el primer motivo en el orden
// MOV_SHIFT_GAP (hueco), MOV_SHIFT o MOV_JUMP (salto), MOV_ABRUPT (brusco).
func openVariant(n entities.MotionNotice) string {
	switch {
	case n.Reason(entities.MotionShiftGap) != nil:
		return messages.VariantGap
	case n.Reason(entities.MotionShift) != nil || n.Reason(entities.MotionJump) != nil:
		return messages.VariantJump
	case n.Reason(entities.MotionAbrupt) != nil:
		return messages.VariantAbrupt
	}
	return messages.VariantJump
}

// motionStartedAt (§7.5): con MOV_ABRUPT, t − (hours − 1) h; con
// MOV_SHIFT_GAP, el fin de la hora que contiene gap_from; si no, t.
func motionStartedAt(n entities.MotionNotice, at time.Time) time.Time {
	switch openVariant(n) {
	case messages.VariantGap:
		if r := n.Reason(entities.MotionShiftGap); r.GapFrom != nil {
			return r.GapFrom.UTC().Truncate(time.Hour).Add(time.Hour)
		}
	case messages.VariantAbrupt:
		r := n.Reason(entities.MotionAbrupt)
		return at.Add(-time.Duration(max(r.Hours, 1)-1) * time.Hour)
	}
	return at
}

// pair devuelve pitch y roll del primer motivo que los trae (SHIFT_GAP,
// SHIFT, JUMP); con MOV_ABRUPT, el delta en su eje; si no, el desplazamiento.
func pair(n entities.MotionNotice) (float64, float64) {
	for _, code := range []string{entities.MotionShiftGap, entities.MotionShift, entities.MotionJump} {
		if r := n.Reason(code); r != nil && r.DP != nil && r.DR != nil {
			return *r.DP, *r.DR
		}
	}
	if r := n.Reason(entities.MotionAbrupt); r != nil {
		if r.Axis == "roll" {
			return 0, r.Delta
		}
		return r.Delta, 0
	}
	return deref(n.ShiftPitch), deref(n.ShiftRoll)
}

func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func (m *motionBuilder) openParams(variant string) map[string]string {
	n := m.in.Notice
	params := map[string]string{"hora": messages.FormatClock(m.at), "fecha": messages.FormatDate(m.at)}
	switch variant {
	case messages.VariantGap:
		r := n.Reason(entities.MotionShiftGap)
		if r.GapTo != nil {
			params["hora"], params["fecha"] = messages.FormatClock(*r.GapTo), messages.FormatDate(*r.GapTo)
		}
		hours := 0
		if r.GapFrom != nil && r.GapTo != nil {
			hours = int(math.Round(r.GapTo.Sub(*r.GapFrom).Hours()))
		}
		params["duracion_hueco"] = messages.FormatDuration(hours)
		params["dp"], params["dr"] = messages.FormatDegrees(deref(r.DP)), messages.FormatDegrees(deref(r.DR))
	case messages.VariantAbrupt:
		r := n.Reason(entities.MotionAbrupt)
		params["eje"], params["delta"], params["horas"] = r.Axis, messages.FormatDegrees(r.Delta), strconv.Itoa(r.Hours)
	default:
		dp, dr := pair(n)
		params["dp"], params["dr"] = messages.FormatDegrees(dp), messages.FormatDegrees(dr)
	}
	return params
}

// open: sin alerta ACTIVA, reactiva la última FINALIZADA (menos de
// merge_window_hours y sin clasificar) o crea una alerta de MOVIMIENTO en Amarillo.
func (m *motionBuilder) open(d *Decision) entities.Alert {
	n := m.in.Notice
	p := m.in.Policy
	variant := openVariant(n)
	started := motionStartedAt(n, m.at)
	at := m.at

	last := m.in.Last
	withinWindow := last != nil && last.EndedAt != nil && last.State == entities.AlertFinished &&
		m.at.Sub(*last.EndedAt) < time.Duration(p.MergeWindowHours)*time.Hour
	if withinWindow && (!last.Management.Classified() || !p.ReactivateOnlyIfUnclassified) {
		a := cloneAlert(*last)
		paused := last.Runtime.PausedHours + max(int(started.Sub(*a.EndedAt).Hours()), 0)
		level := entities.MaxLevel(a.CurrentLevel, entities.LevelAmarillo)
		a.AddEvent(entities.EventTypeMovement)
		a.State = entities.AlertActive
		a.EndedAt = nil
		a.ReactivationCount++
		a.LastReactivatedAt = &at
		a.CurrentLevel = level
		a.MaxLevel = entities.MaxLevel(a.MaxLevel, level)
		a.BelowLevelHours = 0
		a.Runtime = entities.Runtime{PausedHours: paused, MotionAt: &at}
		d.Messages = append(d.Messages, m.message(&a, messages.Key(messages.CodeMovOpen, variant), m.openParams(variant), entities.KindNotification, level, false))
		d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentReactivation, Level: level, StartedAt: started}
		d.State.LastAlertID = nil
		d.Outcome = OutcomeReactivated
		return a
	}

	a := entities.Alert{
		MeasuringPointID: n.MeasuringPointID,
		EventType:        entities.EventTypeMovement,
		Events:           []entities.EventType{entities.EventTypeMovement},
		State:            entities.AlertActive,
		StartedAt:        started,
		OpenedAt:         at,
		CurrentLevel:     entities.LevelAmarillo,
		MaxLevel:         entities.LevelAmarillo,
		LastEvaluatedAt:  at,
		HoursCalculated:  1,
		PolicyVersionID:  p.ID,
		Management:       entities.Management{Status: entities.StatusUnclassified},
		Runtime:          entities.Runtime{MotionAt: &at},
	}
	d.Messages = append(d.Messages, m.message(&a, messages.Key(messages.CodeMovOpen, variant), m.openParams(variant), "", entities.LevelAmarillo, false))
	d.IsNew = true
	d.NewSegment = &entities.Segment{OpenedBy: entities.SegmentOpen, Level: entities.LevelAmarillo, StartedAt: started}
	d.Outcome = OutcomeOpened
	return a
}

// joined: el movimiento abre su episodio con la alerta ACTIVA. El color no cambia.
func (m *motionBuilder) joined(d *Decision, a *entities.Alert) {
	at := m.at
	a.AddEvent(entities.EventTypeMovement)
	a.Runtime.MotionAt = &at
	dp, dr := pair(m.in.Notice)
	d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeMovJoined, ""), map[string]string{
		"hora": messages.FormatClock(at), "fecha": messages.FormatDate(at),
		"dp": messages.FormatDegrees(dp), "dr": messages.FormatDegrees(dr),
	}, "", a.CurrentLevel, false))
	d.Outcome = OutcomeUpdated
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

// displaced: un desplazamiento o cambio brusco nuevo en una hora que no abre.
func (m *motionBuilder) displaced(d *Decision, a *entities.Alert) {
	n := m.in.Notice
	var variant string
	var r *entities.MotionReason
	for _, c := range []struct{ code, variant string }{
		{entities.MotionShiftGap, messages.VariantGap}, {entities.MotionShift, messages.VariantJump}, {entities.MotionAbrupt, messages.VariantAbrupt},
	} {
		if r = n.Reason(c.code); r != nil {
			variant = c.variant
			break
		}
	}
	if r == nil {
		return
	}
	params := map[string]string{}
	if variant == messages.VariantAbrupt {
		params["hora"], params["eje"] = messages.FormatClock(m.at), r.Axis
		params["delta"], params["horas"] = messages.FormatDegrees(r.Delta), strconv.Itoa(r.Hours)
	} else {
		params["hora_inicio"] = messages.FormatHour(shiftStart(*a, n, m.at))
		params["dp"], params["dr"] = messages.FormatDegrees(deref(r.DP)), messages.FormatDegrees(deref(r.DR))
	}
	d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeMovDisplaced, variant), params, "", a.CurrentLevel, false))
}

// temperatureAndReturn: TEMP_STABILIZED, MOV_RETURN, TEMP_VARIES_AGAIN,
// TEMP_NOT_STABILIZED y TEMP_STILL_STABLE, en ese orden.
func (m *motionBuilder) temperatureAndReturn(d *Decision, a *entities.Alert) {
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
	if r := n.Reason(entities.MotionReturn); r != nil {
		back := m.at.Add(-time.Hour)
		if r.StartedAt != nil {
			back = *r.StartedAt
		}
		start := shiftStart(*a, n, m.at)
		d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeMovReturn, ""), map[string]string{
			"hora": messages.FormatClock(back), "hora_inicio": messages.FormatClock(start), "fecha_inicio": messages.FormatDate(start),
			"dp": messages.FormatDegrees(deref(r.DP)), "dr": messages.FormatDegrees(deref(r.DR)),
		}, "", a.CurrentLevel, false))
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
		a.Runtime.TempVariesAgainAt = &at
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

// cleared: con FUGA en episodio la alerta sigue (PART_NORMAL sensor); si no, finaliza.
func (m *motionBuilder) cleared(d *Decision, a *entities.Alert) {
	n := m.in.Notice
	if m.in.LeakInEpisode {
		d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodePartNormal, messages.VariantSensor), map[string]string{
			"nivel": messages.LevelName(a.CurrentLevel), "i": messages.FormatValue(a.LastIndicatorValue),
		}, "", a.CurrentLevel, false))
		return
	}
	at := m.at
	a.State = entities.AlertFinished
	a.EndedAt = &at
	d.CloseSegmentAt = &at
	tramo := ""
	if a.TempStableFrom != nil && a.TempStableTo != nil {
		tramo = " Temperatura estable del " + messages.FormatHour(*a.TempStableFrom) + " al " + messages.FormatHour(*a.TempStableTo) + "."
	}
	newPosition := ""
	if n.Reason(entities.MotionNewPosition) != nil {
		newPosition = " La posición actual del sensor queda como la normal."
	}
	d.Messages = append(d.Messages, m.message(a, messages.Key(messages.CodeEndNormal, messages.VariantMotion), map[string]string{
		"duracion": messages.FormatDuration(a.DurationHours(at)), "max": messages.LevelName(a.MaxLevel),
		"tramo": tramo, "nueva_posicion": newPosition,
	}, "", a.CurrentLevel, false))
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
