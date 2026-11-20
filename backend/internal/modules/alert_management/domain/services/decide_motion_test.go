package services

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// Escenarios de §9.2 (CA-12) con FUGA y MOVIMIENTO como alertas SEPARADAS:
// si se solapan son dos alertas, cada una con sus colores y mensajes. Lo único
// compartido es la temperatura: los TEMP_* salen también en la alerta de FUGA
// vigente. En cada hora, primero FUGA y después MOVIMIENTO, como el backfill.

// cworld agrega al mundo de Decide (alerta de FUGA en world.active) el flujo
// MOVIMIENTO con su propia alerta activa.
type cworld struct {
	*world
	motion       entities.ProcessingState
	motionActive *entities.Alert
}

func newCWorld(t *testing.T) *cworld { return &cworld{world: newWorld(t)} }

// lastFinished es la última alerta FINALIZADA del (punto, evento) (GetLastFinished).
func (w *cworld) lastFinished(e entities.EventType) *entities.Alert {
	var last *entities.Alert
	for _, a := range w.alerts {
		if a.EventType == e && a.State == entities.AlertFinished && a.EndedAt != nil && (last == nil || a.EndedAt.After(*last.EndedAt)) {
			c := *a
			last = &c
		}
	}
	return last
}

func (w *cworld) copyOf(a *entities.Alert) (*entities.Alert, *entities.Segment) {
	if a == nil {
		return nil, nil
	}
	c := *a
	return &c, w.openSegment(c.ID)
}

func (w *cworld) leak(ns ...entities.LevelNotice) {
	w.t.Helper()
	for _, n := range ns {
		in := DecideInput{State: w.state, Notice: n, Policy: w.policy, Device: entities.Device{MeasuringPointID: pointID, PK: "PK34+470"}}
		in.Active, in.Segment = w.copyOf(w.active)
		if in.Active == nil {
			in.Last = w.lastFinished(entities.EventTypeLeak)
		}
		d, err := Decide(in)
		if err != nil {
			w.t.Fatalf("FUGA %v: %v", n.EvaluatedAt, err)
		}
		w.apply(d)
	}
}

// applyOther guarda una decisión que no es de FUGA sin tocar el estado ni la
// alerta activa de FUGA; devuelve la alerta tal como quedó (nil si finalizó).
func (w *cworld) applyOther(d Decision) *entities.Alert {
	leakState, leakActive := w.state, w.active
	w.apply(d)
	out := w.active
	w.state, w.active = leakState, leakActive
	return out
}

func (w *cworld) mov(ns ...entities.MotionNotice) {
	w.t.Helper()
	for _, n := range ns {
		in := MotionInput{State: w.motion, Notice: n, Policy: w.policy, Device: entities.Device{MeasuringPointID: pointID, PK: "PK34+470"}}
		in.Active, in.Segment = w.copyOf(w.motionActive)
		if in.Active == nil {
			in.Last = w.lastFinished(entities.EventTypeMovement)
		}
		d, err := DecideMotion(in)
		if err != nil {
			w.t.Fatalf("MOVIMIENTO %v: %v", n.EvaluatedAt, err)
		}
		if d.Alert != nil {
			w.motionActive = w.applyOther(d)
		}
		w.motion = d.State

		// La temperatura también va a la alerta de FUGA vigente.
		if w.active != nil {
			td, err := DecideTemperature(TemperatureInput{Alert: *w.active, Notice: n, Policy: w.policy})
			if err != nil {
				w.t.Fatalf("TEMPERATURA %v: %v", n.EvaluatedAt, err)
			}
			if td.Alert != nil {
				leakState := w.state
				w.apply(td)
				w.state = leakState
			}
		}
	}
}

// hour alimenta, para cada hora, FUGA y después la hora de movimiento.
func (w *cworld) hour(leak entities.LevelNotice, motion entities.MotionNotice) {
	w.t.Helper()
	w.leak(leak)
	w.mov(motion)
}

// kinds son "CODIGO:KIND" de los mensajes, en orden.
func (w *cworld) kinds() []string {
	out := make([]string, len(w.messages))
	for i, m := range w.messages {
		out[i] = m.Code + ":" + string(m.Kind)
	}
	return out
}

// kindsOf son los "CODIGO:KIND" de las alertas del evento, en orden.
func (w *cworld) kindsOf(e entities.EventType) []string {
	var out []string
	for _, m := range w.messages {
		if w.alerts[m.AlertID].EventType == e {
			out = append(out, m.Code+":"+string(m.Kind))
		}
	}
	return out
}

// alertsOf son las alertas del evento, por ID.
func (w *cworld) alertsOf(e entities.EventType) []entities.Alert {
	var out []entities.Alert
	for id := int64(123); id < w.nextID; id++ {
		if a := w.alerts[id]; a != nil && a.EventType == e {
			out = append(out, *a)
		}
	}
	return out
}

func (w *cworld) message(code string) entities.Message {
	w.t.Helper()
	for _, m := range w.messages {
		if m.Code == code {
			return m
		}
	}
	w.t.Fatalf("no hay %s en %v", code, w.codes())
	return entities.Message{}
}

// messageOf es el primer mensaje con ese código en una alerta del evento.
func (w *cworld) messageOf(e entities.EventType, code string) entities.Message {
	w.t.Helper()
	for _, m := range w.messages {
		if m.Code == code && w.alerts[m.AlertID].EventType == e {
			return m
		}
	}
	w.t.Fatalf("no hay %s en la alerta de %s: %v", code, e, w.codes())
	return entities.Message{}
}

// subsequence verifica que want aparezca en got en ese orden.
func subsequence(got, want []string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

// withoutTemp quita los TEMP_* (lo único que el movimiento agrega a FUGA).
func withoutTemp(kinds []string) []string {
	return slices.DeleteFunc(slices.Clone(kinds), func(k string) bool { return strings.HasPrefix(k, "TEMP_") })
}

// leakAlone son los mensajes que FUGA da sin ninguna hora de movimiento.
func leakAlone(t *testing.T, ns ...entities.LevelNotice) []string {
	w := newCWorld(t)
	w.leak(ns...)
	return w.kinds()
}

// ---- horas de prueba ----

const (
	notif  = "NOTIFICACION"
	update = "ACTUALIZACION"
	fin    = "FIN"
)

func quietLeak(n int) entities.LevelNotice { return calc(n, 0.05, N, N, none, inEpisode(false)) }

func f64p(v float64) *float64 { return &v }
func tp(t time.Time) *time.Time {
	return &t
}

// mv es una hora de movimiento calculada.
func mv(n int, ev entities.EpisodeEvent, inEp bool, motionState string, reasons ...entities.MotionReason) entities.MotionNotice {
	level := N
	if inEp {
		level = A
	}
	return entities.MotionNotice{
		MotionEvaluationID: int64(1000 + n), MeasuringPointID: pointID, EvaluatedAt: h(n),
		Status: entities.StatusCalculated, MotionState: motionState, TempState: "TEMP_VARIABLE",
		TempMode: "MOVIMIENTO_Y_TEMPERATURA", ComputedLevel: level, InEpisode: inEp, EpisodeEvent: ev,
		Reasons: reasons, AlertsEnabled: true,
	}
}

func withTemp(n entities.MotionNotice, state string) entities.MotionNotice {
	n.TempState = state
	return n
}

func displacedAt(n entities.MotionNotice, started time.Time) entities.MotionNotice {
	n.MotionState = entities.MotionDisplaced
	n.ShiftStartedAt, n.ShiftPitch, n.ShiftRoll = tp(started), f64p(0.07), f64p(0.14)
	return n
}

func rJump() entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionJump, DP: f64p(0.02), DR: f64p(0.035), Deriva: f64p(0.05)}
}
func rShift(started time.Time) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionShift, DP: f64p(0.06), DR: f64p(0.115), StartedAt: tp(started)}
}
func rGap(from, to time.Time) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionShiftGap, DP: f64p(0.1), DR: f64p(0.08), GapFrom: tp(from), GapTo: tp(to)}
}
func rAbrupt(hours int) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionAbrupt, Axis: "roll", Delta: 0.04, Hours: hours}
}
func rStab(from time.Time) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionTempStabilized, TempRateMax6h: f64p(0.025), TempStableFrom: tp(from)}
}
func rReturn(started time.Time) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionReturn, DP: f64p(-0.07), DR: f64p(-0.12), StartedAt: tp(started)}
}
func rVaries(from, to time.Time) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionTempVariesAgain, Rate: 0.445, TempStableFrom: tp(from), TempStableTo: tp(to)}
}
func rNotStab() entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionTempNotStabilized, Hours: 36}
}
func rStillStable(hours int) entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionTempStillStable, Hours: hours, TempRateMax6h: f64p(0.02)}
}
func rStill() entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionStill, Hours: 24}
}
func rNewPos() entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionNewPosition}
}

// quietMotion es una hora de movimiento normal fuera de episodio.
func quietMotion(n int) entities.MotionNotice { return mv(n, none, false, "NORMAL") }

// motionEpisode es una hora de movimiento dentro del episodio, sin motivos.
func motionEpisode(n int, state string) entities.MotionNotice { return mv(n, none, true, state) }

// ---- escenarios ----

// 1. Solo ruido: idéntico a hoy aunque lleguen las horas de movimiento (CA-1).
func TestScenario01_OnlyNoiseIsUnchanged(t *testing.T) {
	leak := []entities.LevelNotice{
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.27, O, O, none),
		calc(3, 0.12, N, N, cleared, belowRun(1)),
	}
	alone := newWorld(t)
	alone.feed(leak...)

	w := newCWorld(t)
	for i, n := range leak {
		w.hour(n, quietMotion(i))
	}
	if !slices.Equal(w.codes(), alone.codes()) || len(w.alerts) != 1 {
		t.Fatalf("con movimiento %v, sin movimiento %v", w.codes(), alone.codes())
	}
	if a := w.alerts[123]; a.State != entities.AlertFinished || a.EventType != entities.EventTypeLeak || a.TempState != "" {
		t.Fatalf("alerta %+v", a)
	}
}

// 2. Cambio brusco de un eje: MOV_OPEN brusco y END_NORMAL a las 24 h.
func TestScenario02_AbruptOneAxis(t *testing.T) {
	w := newCWorld(t)
	w.hour(quietLeak(0), mv(0, opened, true, "BRUSCO", rAbrupt(3)))
	for n := 1; n < 24; n++ {
		w.hour(quietLeak(n), motionEpisode(n, "NORMAL"))
	}
	w.hour(quietLeak(24), mv(24, cleared, false, "NORMAL", rStill()))

	if want := []string{"MOV_OPEN:" + notif, "END_NORMAL:" + fin}; !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	a := w.alerts[123]
	if a.EventType != entities.EventTypeMovement || a.MaxLevel != A || !a.StartedAt.Equal(h(-2)) || !a.EndedAt.Equal(h(24)) {
		t.Fatalf("alerta %+v", a)
	}
	if open := w.message("MOV_OPEN"); open.Title != "Movimiento brusco del sensor" || !strings.Contains(open.Body, "roll +0,04° en 3 h") {
		t.Fatalf("MOV_OPEN %q / %q", open.Title, open.Body)
	}
	if end := w.message("END_NORMAL"); end.Title != "Volvió a la normalidad" || end.Body != "Duró 26 h, nivel máximo Amarillo." {
		t.Fatalf("END_NORMAL %q / %q", end.Title, end.Body)
	}
}

// 3. Movimiento lento: ninguna hora entra en episodio; no hay alerta.
func TestScenario03_SlowMovementNoAlert(t *testing.T) {
	w := newCWorld(t)
	for n := 0; n < 30; n++ {
		w.hour(quietLeak(n), quietMotion(n))
	}
	if len(w.alerts) != 0 || len(w.messages) != 0 {
		t.Fatalf("alertas %d mensajes %v", len(w.alerts), w.codes())
	}
}

// salto hace la secuencia de un salto con temperatura estable y regreso.
func jumpStableReturn(w *cworld, start int) (end int) {
	started := h(start - 1).Add(15*time.Minute + 42*time.Second)
	w.hour(quietLeak(start), mv(start, opened, true, "SALTO", rJump()))
	w.hour(quietLeak(start+1), displacedAt(mv(start+1, none, true, "", rShift(started)), started))
	for n := start + 2; n < start+14; n++ {
		w.hour(quietLeak(n), withTemp(displacedAt(motionEpisode(n, ""), started), "VERIFICANDO"))
	}
	w.hour(quietLeak(start+14), withTemp(displacedAt(mv(start+14, none, true, "", rStab(started)), started), entities.TempEstable))
	back := h(start + 40).Add(-15 * time.Minute)
	w.hour(quietLeak(start+41), mv(start+41, none, true, "RETORNO", rReturn(back), rJump(), rVaries(started, back)))
	for n := start + 42; n < start+65; n++ {
		w.hour(quietLeak(n), motionEpisode(n, "NORMAL"))
	}
	w.hour(quietLeak(start+65), mv(start+65, cleared, false, "NORMAL", rStill()))
	return start + 65
}

// 4. Salto, temperatura estable y regreso, sin ruido.
func TestScenario04_JumpStableReturnWithoutNoise(t *testing.T) {
	w := newCWorld(t)
	jumpStableReturn(w, 0)
	want := []string{"MOV_OPEN:" + notif, "MOV_DISPLACED:" + update, "TEMP_STABILIZED:" + notif,
		"MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	a := w.alerts[123]
	if a.TempStableFrom == nil || a.TempStableTo == nil || a.State != entities.AlertFinished {
		t.Fatalf("alerta %+v", a)
	}
	if ret := w.message("MOV_RETURN"); !strings.Contains(ret.Body, "a la posición que tenía antes de las 04:15 del 24/08") {
		t.Fatalf("MOV_RETURN %q", ret.Body)
	}
	if end := w.message("END_NORMAL"); !strings.Contains(end.Body, " Temperatura estable del 04:15 del 24/08 al ") {
		t.Fatalf("END_NORMAL %q", end.Body)
	}
	if v := w.message("TEMP_VARIES_AGAIN"); strings.Contains(v.Body, "no volvió") {
		t.Fatalf("con regreso no dice que no volvió: %q", v.Body)
	}
}

// 5. Ruido y después salto (agosto, real): dos alertas. La de FUGA sigue
// sus reglas de siempre (finaliza con su CLEARED) y recibe los TEMP_*; la de
// MOVIMIENTO abre con MOV_OPEN aunque la de FUGA esté activa.
func TestScenario05_NoiseThenJump(t *testing.T) {
	leak := []entities.LevelNotice{
		calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.27, O, O, none), calc(3, 0.40, R, R, none), calc(4, 0.40, R, R, none), calc(5, 0.40, R, R, none),
		calc(6, 0.12, N, N, cleared, belowRun(1)), calc(7, 0.10, N, N, none, inEpisode(false)),
		calc(8, 0.30, O, R, opened, hoursAbove(2)), calc(9, 0.30, O, R, none), calc(10, 0.30, O, R, none),
		calc(11, 0.12, N, N, cleared, belowRun(1)),
	}
	started := h(3).Add(15 * time.Minute)
	back := h(9).Add(-15 * time.Minute)
	motion := []entities.MotionNotice{
		quietMotion(0), quietMotion(1), quietMotion(2), quietMotion(3),
		mv(4, opened, true, "SALTO", rJump()),
		displacedAt(mv(5, none, true, "", rShift(started)), started),
		withTemp(displacedAt(motionEpisode(6, ""), started), "VERIFICANDO"),
		withTemp(displacedAt(mv(7, none, true, "", rStab(started)), started), entities.TempEstable),
		withTemp(displacedAt(motionEpisode(8, ""), started), entities.TempEstable),
		mv(9, none, true, "RETORNO", rReturn(back), rVaries(started, back)),
		mv(10, cleared, false, "NORMAL", rStill()),
		quietMotion(11),
	}
	w := newCWorld(t)
	for i := range leak {
		w.hour(leak[i], motion[i])
	}

	// FUGA: lo mismo que sin movimiento, más los TEMP_* de su tramo activo.
	if got, alone := withoutTemp(w.kindsOf(entities.EventTypeLeak)), leakAlone(t, leak...); !slices.Equal(got, alone) {
		t.Fatalf("FUGA con movimiento %v\nsin movimiento %v", got, alone)
	}
	// La FUGA finaliza en h6 y se reactiva en h8 (regla de 24 h de siempre).
	leaks := w.alertsOf(entities.EventTypeLeak)
	if len(leaks) != 1 || leaks[0].ReactivationCount != 1 || !leaks[0].EndedAt.Equal(h(11)) || leaks[0].MaxLevel != R {
		t.Fatalf("alertas de FUGA %+v", leaks)
	}
	// TEMP_STABILIZED (h7) cae con la FUGA finalizada: no le llega. TEMP_VARIES_AGAIN (h9), sí.
	leakKinds := w.kindsOf(entities.EventTypeLeak)
	if slices.Contains(leakKinds, "TEMP_STABILIZED:"+notif) || !slices.Contains(leakKinds, "TEMP_VARIES_AGAIN:"+notif) {
		t.Fatalf("FUGA %v", leakKinds)
	}
	if leaks[0].TempStableFrom == nil || leaks[0].TempStableTo == nil {
		t.Fatalf("la FUGA vigente no guardó el tramo de temperatura: %+v", leaks[0])
	}

	// MOVIMIENTO: su propia alerta, Amarillo, de principio a fin.
	want := []string{"MOV_OPEN:" + notif, "MOV_DISPLACED:" + update, "TEMP_STABILIZED:" + notif,
		"MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if got := w.kindsOf(entities.EventTypeMovement); !slices.Equal(got, want) {
		t.Fatalf("MOVIMIENTO %v", got)
	}
	moves := w.alertsOf(entities.EventTypeMovement)
	if len(moves) != 1 || moves[0].MaxLevel != A || !moves[0].EndedAt.Equal(h(10)) || moves[0].StartedAt.Before(h(4)) {
		t.Fatalf("alerta de MOVIMIENTO %+v", moves)
	}
	for _, m := range w.messages {
		if a := w.alerts[m.AlertID]; a.EventType == entities.EventTypeLeak && strings.HasPrefix(m.Code, "MOV_") {
			t.Fatalf("mensaje del sensor en la alerta de FUGA: %s", m.Code)
		}
	}
}

// 6. Movimiento brusco y después ruido: la FUGA abre su propia alerta con
// OPEN (no se une a la de MOVIMIENTO) y cada una finaliza por su cuenta.
func TestScenario06_AbruptThenNoise(t *testing.T) {
	leak := []entities.LevelNotice{
		quietLeak(0), calc(1, 0.17, A, A, none, inEpisode(false)), calc(2, 0.18, A, A, opened, hoursAbove(2)),
		calc(3, 0.27, O, O, none), calc(4, 0.27, O, O, none), calc(5, 0.12, N, N, cleared, belowRun(1)),
	}
	motion := []entities.MotionNotice{
		mv(0, opened, true, "BRUSCO", rAbrupt(2)), motionEpisode(1, "BRUSCO"), motionEpisode(2, "NORMAL"),
		motionEpisode(3, "NORMAL"), mv(4, cleared, false, "NORMAL", rStill()), quietMotion(5),
	}
	w := newCWorld(t)
	for i := range leak {
		w.hour(leak[i], motion[i])
	}

	if got := w.kindsOf(entities.EventTypeMovement); !slices.Equal(got, []string{"MOV_OPEN:" + notif, "END_NORMAL:" + fin}) {
		t.Fatalf("MOVIMIENTO %v", got)
	}
	if got, alone := w.kindsOf(entities.EventTypeLeak), leakAlone(t, leak...); !slices.Equal(got, alone) || got[0] != "OPEN:"+notif {
		t.Fatalf("FUGA %v, sola %v", got, alone)
	}
	m, l := w.alerts[123], w.alerts[124]
	if m.EventType != entities.EventTypeMovement || m.MaxLevel != A || !m.EndedAt.Equal(h(4)) {
		t.Fatalf("MOVIMIENTO %+v", m)
	}
	if l.EventType != entities.EventTypeLeak || l.MaxLevel != O || !l.EndedAt.Equal(h(5)) {
		t.Fatalf("FUGA %+v", l)
	}
}

// 7. Ruido y movimiento en la misma hora: dos alertas que abren a la vez.
func TestScenario07_SameHour(t *testing.T) {
	w := newCWorld(t)
	w.hour(calc(0, 0.17, A, A, none, inEpisode(false)), quietMotion(0))
	w.hour(calc(1, 0.18, A, A, opened, hoursAbove(2)), mv(1, opened, true, "SALTO", rJump()))
	w.hour(calc(2, 0.12, N, N, cleared, belowRun(1)), motionEpisode(2, "NORMAL"))
	for n := 3; n < 25; n++ {
		w.hour(quietLeak(n), motionEpisode(n, "NORMAL"))
	}
	w.hour(quietLeak(25), mv(25, cleared, false, "NORMAL", rStill()))

	if got := w.kindsOf(entities.EventTypeLeak); got[0] != "OPEN:"+notif || got[len(got)-1] != "END_NORMAL:"+fin {
		t.Fatalf("FUGA %v", got)
	}
	if got := w.kindsOf(entities.EventTypeMovement); !slices.Equal(got, []string{"MOV_OPEN:" + notif, "END_NORMAL:" + fin}) {
		t.Fatalf("MOVIMIENTO %v", got)
	}
	if l, m := w.alerts[123], w.alerts[124]; !l.EndedAt.Equal(h(2)) || !m.EndedAt.Equal(h(25)) {
		t.Fatalf("fines FUGA %v MOVIMIENTO %v", l.EndedAt, m.EndedAt)
	}
}

// 8. Ruido mientras la temperatura está estable: la FUGA abre su alerta con
// OPEN y recibe el TEMP_VARIES_AGAIN mientras está vigente.
func TestScenario08_NoiseWhileTemperatureStable(t *testing.T) {
	w := newCWorld(t)
	started := h(-1).Add(15 * time.Minute)
	w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
	w.hour(quietLeak(1), withTemp(displacedAt(mv(1, none, true, "", rStab(started)), started), entities.TempEstable))
	stable := func(n int) entities.MotionNotice {
		return withTemp(displacedAt(motionEpisode(n, ""), started), entities.TempEstable)
	}
	w.hour(calc(2, 0.17, A, A, none, inEpisode(false)), stable(2))
	w.hour(calc(3, 0.18, A, A, opened, hoursAbove(2)), stable(3))
	w.hour(calc(4, 0.27, O, O, none), stable(4))
	for n := 5; n < 12; n++ {
		w.hour(calc(n, 0.27, O, O, none), stable(n))
	}
	back := h(12).Add(-15 * time.Minute)
	w.hour(calc(12, 0.27, O, O, none), mv(12, none, true, "RETORNO", rReturn(back), rVaries(started, back)))
	w.hour(calc(13, 0.12, N, N, cleared, belowRun(1)), motionEpisode(13, "NORMAL"))
	for n := 14; n < 36; n++ {
		w.hour(quietLeak(n), motionEpisode(n, "NORMAL"))
	}
	w.hour(quietLeak(36), mv(36, cleared, false, "NORMAL", rStill()))

	want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if got := w.kindsOf(entities.EventTypeMovement); !slices.Equal(got, want) {
		t.Fatalf("MOVIMIENTO %v", got)
	}
	leakKinds := w.kindsOf(entities.EventTypeLeak)
	if !subsequence(leakKinds, []string{"OPEN:" + notif, "UP_NARANJA_FAST:" + notif, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}) ||
		slices.Contains(leakKinds, "TEMP_STABILIZED:"+notif) || slices.Contains(leakKinds, "MOV_RETURN:"+update) {
		t.Fatalf("FUGA %v", leakKinds)
	}
	if got := w.levels(124); !slices.Equal(got, []entities.Level{A, O}) {
		t.Fatalf("colores FUGA %v", got)
	}
	// Mismo texto en las dos alertas.
	if lv, mv := w.messageOf(entities.EventTypeLeak, "TEMP_VARIES_AGAIN"), w.messageOf(entities.EventTypeMovement, "TEMP_VARIES_AGAIN"); lv.Body != mv.Body || lv.Kind != mv.Kind || lv.Level != O {
		t.Fatalf("TEMP_VARIES_AGAIN FUGA %+v / MOVIMIENTO %+v", lv, mv)
	}
}

// 9. Salto sin que la temperatura se estabilice: a las 36 h, TEMP_NOT_STABILIZED y fin en la posición nueva.
func TestScenario09_TemperatureNeverStabilizes(t *testing.T) {
	w := newCWorld(t)
	started := h(-1).Add(15 * time.Minute)
	w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
	for n := 1; n < 36; n++ {
		w.hour(quietLeak(n), withTemp(displacedAt(motionEpisode(n, ""), started), "VERIFICANDO"))
	}
	w.hour(quietLeak(36), withTemp(mv(36, cleared, false, "NORMAL", rNotStab(), rNewPos(), rStill()), "TEMP_VARIABLE"))

	if want := []string{"MOV_OPEN:" + notif, "TEMP_NOT_STABILIZED:" + update, "END_NORMAL:" + fin}; !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	if end := w.message("END_NORMAL"); !strings.HasSuffix(end.Body, " La posición actual del sensor queda como la normal.") {
		t.Fatalf("END_NORMAL %q", end.Body)
	}
}

// 10. El sensor vuelve y la temperatura sigue estable: TEMP_STILL_STABLE a las 6, 18, 30 h.
func TestScenario10_ReturnWithTemperatureStillStable(t *testing.T) {
	w := newCWorld(t)
	started := h(-1).Add(15 * time.Minute)
	w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
	w.hour(quietLeak(1), withTemp(displacedAt(mv(1, none, true, "", rStab(started)), started), entities.TempEstable))
	back := h(4).Add(-15 * time.Minute)
	w.hour(quietLeak(4), withTemp(mv(4, none, true, "RETORNO", rReturn(back)), entities.TempRetornoEstable))
	for _, k := range []int{6, 18, 30} {
		w.hour(quietLeak(4+k), withTemp(mv(4+k, none, true, "NORMAL", rStillStable(k)), entities.TempRetornoEstable))
	}
	w.hour(quietLeak(40), mv(40, none, true, "NORMAL", rVaries(started, back)))
	w.hour(quietLeak(64), mv(64, cleared, false, "NORMAL", rStill()))

	want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "MOV_RETURN:" + update,
		"TEMP_STILL_STABLE:" + update, "TEMP_STILL_STABLE:" + update, "TEMP_STILL_STABLE:" + update,
		"TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	if s := w.message("TEMP_STILL_STABLE"); s.Body != "El sensor volvió hace 6 h, pero la temperatura de la tubería sigue casi quieta. Dato: máximo 0,02 °C por hora (normal: más de 0,08)." {
		t.Fatalf("TEMP_STILL_STABLE %q", s.Body)
	}
}

// 11. La temperatura vuelve a variar y el sensor no regresa.
func TestScenario11_TemperatureVariesWithoutReturn(t *testing.T) {
	w := newCWorld(t)
	started := h(-1).Add(15 * time.Minute)
	w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
	w.hour(quietLeak(1), withTemp(displacedAt(mv(1, none, true, "", rStab(started)), started), entities.TempEstable))
	w.hour(quietLeak(10), displacedAt(mv(10, none, true, "", rVaries(started, h(8))), started))
	w.hour(quietLeak(34), mv(34, cleared, false, "NORMAL", rNewPos(), rStill()))

	if want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}; !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	if v := w.message("TEMP_VARIES_AGAIN"); v.Body != "Estuvo estable 9 h (04:15 del 24/08 – 13:00 del 24/08). El sensor no volvió a su posición anterior. Dato: 0,445 °C en una hora." {
		t.Fatalf("TEMP_VARIES_AGAIN %q", v.Body)
	}
	if end := w.message("END_NORMAL"); !strings.Contains(end.Body, "queda como la normal") {
		t.Fatalf("END_NORMAL %q", end.Body)
	}
}

// 12. Salto durante pérdida de datos (20–21 oct): MOV_OPEN hueco; inicio = fin de la hora de gap_from.
func TestScenario12_JumpDuringDataLoss(t *testing.T) {
	w := newCWorld(t)
	gapFrom, gapTo := h(-23).Add(15*time.Minute+42*time.Second), h(-1).Add(42*time.Second)
	w.hour(quietLeak(0), displacedAt(mv(0, opened, true, "", rGap(gapFrom, gapTo), rJump()), gapFrom))
	w.hour(quietLeak(5), withTemp(displacedAt(mv(5, none, true, "", rStab(gapFrom)), gapFrom), entities.TempEstable))
	back := h(40).Add(-30 * time.Minute)
	w.hour(quietLeak(40), mv(40, none, true, "RETORNO", rReturn(back), rJump(), rVaries(gapFrom, back)))
	w.hour(quietLeak(64), mv(64, cleared, false, "NORMAL", rStill()))

	want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	a := w.alerts[123]
	if !a.StartedAt.Equal(h(-22)) {
		t.Fatalf("started_at = %v, se esperaba el fin de la hora de gap_from (%v)", a.StartedAt, h(-22))
	}
	if open := w.message("MOV_OPEN"); open.Title != "El sensor cambió de posición sin datos" || !strings.Contains(open.Body, "Estuvo 22 h sin transmitir") {
		t.Fatalf("MOV_OPEN %q / %q", open.Title, open.Body)
	}
}

// 13. Clasificada (Falsa alerta + Mantenimiento) y después ruido: igual que
// sin clasificar (CA-4); la FUGA abre su propia alerta.
func TestScenario13_ClassifiedKeepsAllMessages(t *testing.T) {
	run := func(classify bool) []string {
		w := newCWorld(t)
		started := h(-1).Add(15 * time.Minute)
		w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
		if classify {
			w.motionActive.Management = entities.Management{Status: entities.StatusClassifiedAndLabel,
				Classification: entities.ClassificationFalseAlarm, Label: entities.LabelScheduledWork}
		}
		stable := func(n int, r ...entities.MotionReason) entities.MotionNotice {
			return withTemp(displacedAt(mv(n, none, true, "", r...), started), entities.TempEstable)
		}
		w.hour(quietLeak(1), stable(1, rStab(started)))
		w.hour(calc(2, 0.17, A, A, none, inEpisode(false)), stable(2))
		w.hour(calc(3, 0.18, A, A, opened, hoursAbove(2)), stable(3))
		w.hour(calc(4, 0.27, O, O, none), stable(4))
		back := h(5).Add(-15 * time.Minute)
		w.hour(calc(5, 0.27, O, O, none), mv(5, none, true, "RETORNO", rReturn(back), rVaries(started, back)))
		w.hour(calc(6, 0.12, N, N, cleared, belowRun(1)), motionEpisode(6, "NORMAL"))
		w.hour(quietLeak(30), mv(30, cleared, false, "NORMAL", rStill()))
		return w.kinds()
	}
	classified, unclassified := run(true), run(false)
	want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "OPEN:" + notif, "UP_NARANJA_FAST:" + notif,
		"MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if !slices.Equal(classified, unclassified) || !subsequence(classified, want) {
		t.Fatalf("clasificada %v\nsin clasificar %v", classified, unclassified)
	}
}

// 14. Finaliza y vuelve a moverse antes de 24 h: reactiva si está Sin
// clasificar; alerta nueva si está clasificada.
func TestScenario14_MovesAgainWithin24h(t *testing.T) {
	for _, classified := range []bool{false, true} {
		w := newCWorld(t)
		w.hour(quietLeak(0), mv(0, opened, true, "BRUSCO", rAbrupt(1)))
		w.hour(quietLeak(24), mv(24, cleared, false, "NORMAL", rStill()))
		if classified {
			w.alerts[123].Management = entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationFalseAlarm}
		}
		w.hour(quietLeak(28), mv(28, opened, true, "BRUSCO", rAbrupt(1)))
		w.hour(quietLeak(52), mv(52, cleared, false, "NORMAL", rStill()))

		want := []string{"MOV_OPEN:" + notif, "END_NORMAL:" + fin, "MOV_OPEN:" + notif, "END_NORMAL:" + fin}
		if !slices.Equal(w.kinds(), want) {
			t.Fatalf("clasificada=%v mensajes %v", classified, w.kinds())
		}
		if classified {
			if len(w.alerts) != 2 || w.alerts[124] == nil {
				t.Fatalf("clasificada: se esperaba una alerta nueva, hay %d", len(w.alerts))
			}
			continue
		}
		a := w.alerts[123]
		if len(w.alerts) != 1 || a.ReactivationCount != 1 || !a.EndedAt.Equal(h(52)) || w.segments[123][1].OpenedBy != entities.SegmentReactivation {
			t.Fatalf("reactivación %+v", a)
		}
	}
}

// 15. Sin datos con la alerta de movimiento abierta: ningún mensaje del movimiento.
func TestScenario15_NoDataWithMotionAlertOpen(t *testing.T) {
	w := newCWorld(t)
	w.hour(quietLeak(0), mv(0, opened, true, "BRUSCO", rAbrupt(1)))
	for n := 1; n < 10; n++ {
		nc := entities.MotionNotice{MeasuringPointID: pointID, EvaluatedAt: h(n), Status: entities.StatusNoCalculable, InEpisode: true, AlertsEnabled: true}
		w.leak(noCalc(n))
		w.mov(nc)
	}
	if !slices.Equal(w.kinds(), []string{"MOV_OPEN:" + notif}) || w.motionActive == nil || w.active != nil || !w.motion.LastEvaluatedAt.Equal(h(9)) {
		t.Fatalf("mensajes %v MOVIMIENTO activa %v", w.kinds(), w.motionActive != nil)
	}
}

// 16. Temperatura estable sin movimiento (por decidir): hoy nada.
func TestScenario16_StableTemperatureWithoutMotion(t *testing.T) {
	w := newCWorld(t)
	for n := 0; n < 20; n++ {
		w.hour(quietLeak(n), withTemp(quietMotion(n), entities.TempEstable))
	}
	if len(w.alerts) != 0 {
		t.Fatalf("alertas %d", len(w.alerts))
	}
}

// 17. Ruido y movimiento coinciden: dos alertas; ninguna sube por la coincidencia.
func TestScenario17_CoincidenceDoesNotRaise(t *testing.T) {
	w := newCWorld(t)
	w.hour(quietLeak(0), mv(0, opened, true, "BRUSCO", rAbrupt(1)))
	w.hour(calc(1, 0.17, A, A, none, inEpisode(false)), motionEpisode(1, "NORMAL"))
	w.hour(calc(2, 0.18, A, A, opened, hoursAbove(2)), motionEpisode(2, "NORMAL"))
	w.hour(calc(3, 0.19, A, A, none), motionEpisode(3, "NORMAL"))
	if w.motionActive == nil || w.active == nil || w.motionActive.ID == w.active.ID {
		t.Fatalf("se esperaban dos alertas activas: MOVIMIENTO %v FUGA %v", w.motionActive, w.active)
	}
	if !slices.Equal(w.levels(123), []entities.Level{A}) || !slices.Equal(w.levels(124), []entities.Level{A}) {
		t.Fatalf("la coincidencia subió el color: %v / %v", w.levels(123), w.levels(124))
	}
}

// SOLO_MOVIMIENTO: sin TEMP_*; el salto cierra al quedar quieto en la posición
// nueva y el regreso (pasadas 24 h) abre otra alerta.
func TestMotion_MotionOnlyMode(t *testing.T) {
	w := newCWorld(t)
	only := func(n entities.MotionNotice) entities.MotionNotice {
		n.TempState, n.TempMode = "", "SOLO_MOVIMIENTO"
		return n
	}
	w.hour(quietLeak(0), only(mv(0, opened, true, "SALTO", rJump())))
	w.hour(quietLeak(24), only(mv(24, cleared, false, "NORMAL", rNewPos(), rStill())))
	w.hour(quietLeak(60), only(mv(60, opened, true, "SALTO", rJump())))
	for _, m := range w.messages {
		if strings.HasPrefix(m.Code, "TEMP_") {
			t.Fatalf("mensaje de temperatura con SOLO_MOVIMIENTO: %s", m.Code)
		}
	}
	if len(w.alerts) != 2 || w.alerts[123].State != entities.AlertFinished || w.motionActive == nil || w.motionActive.ID != 124 {
		t.Fatalf("alertas %d", len(w.alerts))
	}
	if end := w.message("END_NORMAL"); !strings.Contains(end.Body, "queda como la normal") {
		t.Fatalf("END_NORMAL %q", end.Body)
	}
}

// La reactivación es por evento: un movimiento a menos de 24 h del fin de
// una alerta de FUGA abre una alerta de MOVIMIENTO nueva (no reactiva la de FUGA).
func TestMotion_ReactivationIsPerEvent(t *testing.T) {
	w := newCWorld(t)
	w.leak(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)), calc(2, 0.12, N, N, cleared, belowRun(1)))
	w.mov(mv(5, opened, true, "BRUSCO", rAbrupt(1)))
	l, m := w.alerts[123], w.alerts[124]
	if l.State != entities.AlertFinished || l.ReactivationCount != 0 || m == nil || m.EventType != entities.EventTypeMovement {
		t.Fatalf("FUGA %+v MOVIMIENTO %+v", l, m)
	}
	// Y al revés: un ruido tras el fin de MOVIMIENTO abre una FUGA nueva.
	w.mov(mv(29, cleared, false, "NORMAL", rStill()))
	w.leak(calc(30, 0.17, A, A, none, inEpisode(false)), calc(31, 0.18, A, A, opened, hoursAbove(2)))
	if w.alerts[124].ReactivationCount != 0 || w.active == nil || w.active.EventType != entities.EventTypeLeak || w.active.ID == 124 {
		t.Fatalf("MOVIMIENTO %+v FUGA activa %+v", w.alerts[124], w.active)
	}
}

// Con la FUGA activa, las horas de movimiento sin TEMP_* no le agregan nada,
// y la de FUGA no copia el estado del sensor.
func TestDecideTemperature_OnlyTemperatureReachesLeak(t *testing.T) {
	w := newCWorld(t)
	w.leak(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)))
	started := h(1).Add(15 * time.Minute)
	w.mov(mv(2, opened, true, "SALTO", rJump()), displacedAt(mv(3, none, true, "", rShift(started)), started))
	for n := 2; n < 4; n++ {
		w.leak(calc(n, 0.18, A, A, none))
	}
	if got := w.kindsOf(entities.EventTypeLeak); slices.ContainsFunc(got, func(k string) bool { return !strings.HasPrefix(k, "OPEN") }) {
		t.Fatalf("FUGA recibió mensajes del sensor: %v", got)
	}
	if a := w.active; a.MotionState != "" || a.LastMotionEvaluatedAt != nil {
		t.Fatalf("FUGA copió el estado del sensor: %+v", a)
	}
	w.mov(withTemp(displacedAt(mv(4, none, true, "", rStab(started)), started), entities.TempEstable))
	if a := w.active; a.TempState != entities.TempEstable || a.TempStableFrom == nil || !a.TempStableFrom.Equal(started) {
		t.Fatalf("FUGA sin tramo de temperatura: %+v", a)
	}
	stab := w.messageOf(entities.EventTypeLeak, "TEMP_STABILIZED")
	if stab.Kind != entities.KindNotification || stab.Level != A || stab.AlertID != w.active.ID {
		t.Fatalf("TEMP_STABILIZED en FUGA %+v", stab)
	}
}

func TestDecideTemperature_Guards(t *testing.T) {
	leak := entities.Alert{ID: 1, MeasuringPointID: pointID, EventType: entities.EventTypeLeak, State: entities.AlertActive, CurrentLevel: A}
	started := h(0)
	stab := withTemp(displacedAt(mv(1, none, true, "", rStab(started)), started), entities.TempEstable)

	for _, bad := range []entities.Alert{
		func() entities.Alert { a := leak; a.EventType = entities.EventTypeMovement; return a }(),
		func() entities.Alert { a := leak; a.State = entities.AlertFinished; return a }(),
		func() entities.Alert { a := leak; a.MeasuringPointID = 9; return a }(),
	} {
		if _, err := DecideTemperature(TemperatureInput{Alert: bad, Notice: stab}); !errors.Is(err, ErrNoticeMismatch) {
			t.Fatalf("%+v: err = %v", bad, err)
		}
	}
	if d, err := DecideTemperature(TemperatureInput{Alert: leak, Notice: motionEpisode(1, "DESPLAZADO")}); err != nil || d.Alert != nil || len(d.Messages) != 0 {
		t.Fatalf("sin TEMP_*: %+v %v", d, err)
	}
	nc := entities.MotionNotice{MeasuringPointID: pointID, EvaluatedAt: h(1), Status: entities.StatusNoCalculable}
	if d, err := DecideTemperature(TemperatureInput{Alert: leak, Notice: nc}); err != nil || d.Alert != nil {
		t.Fatalf("NO_CALCULABLE: %+v %v", d, err)
	}
	if _, err := DecideMotion(MotionInput{Active: &leak, Notice: stab}); !errors.Is(err, ErrNoticeMismatch) {
		t.Fatalf("DecideMotion con una alerta de FUGA: %v", err)
	}
}
