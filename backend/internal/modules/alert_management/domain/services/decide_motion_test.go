package services

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

// Escenarios de §9.2 (CA-12): FUGA (Decide) y MOVIMIENTO (DecideMotion)
// sobre la misma alerta del punto. En cada hora, primero FUGA y después
// MOVIMIENTO, como el backfill.

// cworld agrega al mundo de Decide el estado del flujo MOVIMIENTO.
type cworld struct {
	*world
	motion entities.ProcessingState
}

func newCWorld(t *testing.T) *cworld { return &cworld{world: newWorld(t)} }

// lastFinished es la última alerta FINALIZADA del punto (GetLastFinished).
func (w *cworld) lastFinished() *entities.Alert {
	var last *entities.Alert
	for _, a := range w.alerts {
		if a.State == entities.AlertFinished && a.EndedAt != nil && (last == nil || a.EndedAt.After(*last.EndedAt)) {
			c := *a
			last = &c
		}
	}
	return last
}

func (w *cworld) activeCopy() (*entities.Alert, *entities.Segment) {
	if w.active == nil {
		return nil, nil
	}
	a := *w.active
	return &a, w.openSegment(a.ID)
}

func (w *cworld) leak(ns ...entities.LevelNotice) {
	w.t.Helper()
	for _, n := range ns {
		in := DecideInput{State: w.state, Notice: n, Policy: w.policy, Device: entities.Device{MeasuringPointID: pointID, PK: "PK34+470"},
			MotionInEpisode: w.motion.InEpisode}
		in.Active, in.Segment = w.activeCopy()
		if in.Active == nil {
			in.Last = w.lastFinished()
		}
		d, err := Decide(in)
		if err != nil {
			w.t.Fatalf("FUGA %v: %v", n.EvaluatedAt, err)
		}
		w.apply(d)
	}
}

func (w *cworld) mov(ns ...entities.MotionNotice) {
	w.t.Helper()
	for _, n := range ns {
		in := MotionInput{State: w.motion, Notice: n, Policy: w.policy, Device: entities.Device{MeasuringPointID: pointID, PK: "PK34+470"},
			LeakInEpisode: w.state.InEpisode}
		in.Active, in.Segment = w.activeCopy()
		if in.Active == nil {
			in.Last = w.lastFinished()
		}
		d, err := DecideMotion(in)
		if err != nil {
			w.t.Fatalf("MOVIMIENTO %v: %v", n.EvaluatedAt, err)
		}
		leakState := w.state
		w.apply(d)
		w.motion, w.state = d.State, leakState
	}
}

// both alimenta, para cada hora, FUGA normal y la hora de movimiento.
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
	if a := w.alerts[123]; a.State != entities.AlertFinished || !slices.Equal(a.Events, []entities.EventType{entities.EventTypeLeak}) {
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

// 5. Ruido y después salto (agosto, real).
func TestScenario05_NoiseThenJump(t *testing.T) {
	w := newCWorld(t)
	w.leak(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.27, O, O, none), calc(3, 0.40, R, R, none))
	w.mov(quietMotion(0), quietMotion(1), quietMotion(2), quietMotion(3))
	started := h(3).Add(15 * time.Minute)
	w.hour(calc(4, 0.40, R, R, none), mv(4, opened, true, "SALTO", rJump()))
	w.hour(calc(5, 0.40, R, R, none), displacedAt(mv(5, none, true, "", rShift(started)), started))
	w.hour(calc(6, 0.12, N, N, cleared, belowRun(1)), withTemp(displacedAt(motionEpisode(6, ""), started), "VERIFICANDO"))
	w.hour(calc(7, 0.10, N, N, none, inEpisode(false)), withTemp(displacedAt(mv(7, none, true, "", rStab(started)), started), entities.TempEstable))
	w.hour(calc(8, 0.30, O, R, opened, hoursAbove(2)), withTemp(displacedAt(motionEpisode(8, ""), started), entities.TempEstable))
	back := h(9).Add(-15 * time.Minute)
	w.hour(calc(9, 0.30, O, R, none), mv(9, none, true, "RETORNO", rReturn(back), rVaries(started, back)))
	w.hour(calc(10, 0.30, O, R, none), mv(10, cleared, false, "NORMAL", rStill()))
	w.hour(calc(11, 0.12, N, N, cleared, belowRun(1)), quietMotion(11))

	want := []string{"OPEN:" + notif, "UP_NARANJA_FAST:" + notif, "UP_ROJO:" + notif, "MOV_JOINED:" + update,
		"MOV_DISPLACED:" + update, "PART_NORMAL:" + update, "TEMP_STABILIZED:" + notif, "LEAK_JOINED:" + notif,
		"MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "PART_NORMAL:" + update, "END_NORMAL:" + fin}
	if !subsequence(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	a := w.alerts[123]
	if len(w.alerts) != 1 || a.State != entities.AlertFinished || !a.EndedAt.Equal(h(11)) || a.MaxLevel != R ||
		!slices.Equal(a.Events, []entities.EventType{entities.EventTypeLeak, entities.EventTypeMovement}) {
		t.Fatalf("alerta %+v", a)
	}
	// Entre el fin del ruido y su regreso el color sigue en Rojo y no hay mensajes horarios de ruido.
	if got := w.levels(123); !slices.Equal(got, []entities.Level{A, O, R}) {
		t.Fatalf("colores %v", got)
	}
	if p := w.messages[slices.Index(w.codes(), "PART_NORMAL")]; p.Body != "La alerta sigue abierta porque el sensor sigue fuera de su posición. Dato: 0,12." {
		t.Fatalf("PART_NORMAL ruido %q", p.Body)
	}
	if j := w.message("LEAK_JOINED"); !strings.Contains(j.Body, "demasiado alto") || !strings.Contains(j.Body, "la temperatura está estable desde las 08:15 del 24/08") {
		t.Fatalf("LEAK_JOINED %q", j.Body)
	}
}

// 6. Movimiento brusco y después ruido.
func TestScenario06_AbruptThenNoise(t *testing.T) {
	w := newCWorld(t)
	w.hour(quietLeak(0), mv(0, opened, true, "BRUSCO", rAbrupt(2)))
	w.hour(calc(1, 0.17, A, A, none, inEpisode(false)), motionEpisode(1, "BRUSCO"))
	w.hour(calc(2, 0.18, A, A, opened, hoursAbove(2)), motionEpisode(2, "NORMAL"))
	w.hour(calc(3, 0.27, O, O, none), motionEpisode(3, "NORMAL"))
	w.hour(calc(4, 0.27, O, O, none), mv(4, cleared, false, "NORMAL", rStill()))
	w.hour(calc(5, 0.12, N, N, cleared, belowRun(1)), quietMotion(5))

	want := []string{"MOV_OPEN:" + notif, "LEAK_JOINED:" + notif, "UP_NARANJA_FAST:" + notif, "PART_NORMAL:" + update, "END_NORMAL:" + fin}
	if !subsequence(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	a := w.alerts[123]
	if a.EventType != entities.EventTypeMovement || !a.EndedAt.Equal(h(5)) || a.MaxLevel != O {
		t.Fatalf("alerta %+v", a)
	}
	if end := w.message("END_NORMAL"); !strings.HasPrefix(end.Title, "La alerta ") {
		t.Fatalf("el fin por ruido es el de FUGA: %q", end.Title)
	}
}

// 7. Ruido y movimiento en la misma hora.
func TestScenario07_SameHour(t *testing.T) {
	w := newCWorld(t)
	w.hour(calc(0, 0.17, A, A, none, inEpisode(false)), quietMotion(0))
	w.hour(calc(1, 0.18, A, A, opened, hoursAbove(2)), mv(1, opened, true, "SALTO", rJump()))
	w.hour(calc(2, 0.12, N, N, cleared, belowRun(1)), motionEpisode(2, "NORMAL"))
	for n := 3; n < 25; n++ {
		w.hour(quietLeak(n), motionEpisode(n, "NORMAL"))
	}
	w.hour(quietLeak(25), mv(25, cleared, false, "NORMAL", rStill()))

	want := []string{"OPEN:" + notif, "MOV_JOINED:" + update, "PART_NORMAL:" + update, "END_NORMAL:" + fin}
	if !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	if end := w.message("END_NORMAL"); end.Title != "Volvió a la normalidad" || !w.alerts[123].EndedAt.Equal(h(25)) {
		t.Fatalf("fin %q en %v", end.Title, w.alerts[123].EndedAt)
	}
}

// 8. Ruido mientras la temperatura está estable.
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
	w.hour(calc(5, 0.12, N, N, cleared, belowRun(1)), stable(5))
	for n := 6; n < 12; n++ {
		w.hour(quietLeak(n), stable(n))
	}
	back := h(12).Add(-15 * time.Minute)
	w.hour(quietLeak(12), mv(12, none, true, "RETORNO", rReturn(back), rVaries(started, back)))
	for n := 13; n < 36; n++ {
		w.hour(quietLeak(n), motionEpisode(n, "NORMAL"))
	}
	w.hour(quietLeak(36), mv(36, cleared, false, "NORMAL", rStill()))

	want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "LEAK_JOINED:" + notif, "UP_NARANJA_FAST:" + notif,
		"PART_NORMAL:" + update, "DOWNGRADE:" + update, "MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
	if !slices.Equal(w.kinds(), want) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	if got := w.levels(123); !slices.Equal(got, []entities.Level{A, O, A}) {
		t.Fatalf("colores %v (la bajada nunca pasa de Amarillo)", got)
	}
	if j := w.message("LEAK_JOINED"); !strings.Contains(j.Body, "El sensor se movió y la temperatura está estable desde las 04:15 del 24/08.") {
		t.Fatalf("LEAK_JOINED sin contexto de temperatura: %q", j.Body)
	}
	if p := w.message("PART_NORMAL"); !strings.Contains(p.Body, "el sensor se movió y la temperatura sigue estable") {
		t.Fatalf("PART_NORMAL %q", p.Body)
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

// 13. Clasificada (Falsa alerta + Mantenimiento) y después ruido: igual que sin clasificar (CA-4).
func TestScenario13_ClassifiedKeepsAllMessages(t *testing.T) {
	run := func(classify bool) []string {
		w := newCWorld(t)
		started := h(-1).Add(15 * time.Minute)
		w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
		if classify {
			w.active.Management = entities.Management{Status: entities.StatusClassifiedAndLabel,
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
	want := []string{"MOV_OPEN:" + notif, "TEMP_STABILIZED:" + notif, "LEAK_JOINED:" + notif, "UP_NARANJA_FAST:" + notif,
		"MOV_RETURN:" + update, "TEMP_VARIES_AGAIN:" + notif, "END_NORMAL:" + fin}
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
	if !slices.Equal(w.kinds(), []string{"MOV_OPEN:" + notif}) || w.active == nil || !w.motion.LastEvaluatedAt.Equal(h(9)) {
		t.Fatalf("mensajes %v activa %v", w.kinds(), w.active != nil)
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

// 17. Ruido y movimiento coinciden (por decidir): igual que el 6; el color no sube por la coincidencia.
func TestScenario17_CoincidenceDoesNotRaise(t *testing.T) {
	w := newCWorld(t)
	w.hour(quietLeak(0), mv(0, opened, true, "BRUSCO", rAbrupt(1)))
	w.hour(calc(1, 0.17, A, A, none, inEpisode(false)), motionEpisode(1, "NORMAL"))
	w.hour(calc(2, 0.18, A, A, opened, hoursAbove(2)), motionEpisode(2, "NORMAL"))
	w.hour(calc(3, 0.19, A, A, none), motionEpisode(3, "NORMAL"))
	if a := w.active; a == nil || a.CurrentLevel != A || !slices.Equal(w.levels(123), []entities.Level{A}) {
		t.Fatalf("la coincidencia subió el color: %v", w.levels(123))
	}
	if j := w.message("LEAK_JOINED"); !strings.Contains(j.Body, "por encima de lo normal") || !strings.Contains(j.Body, "El sensor se movió a las 05:00 del 24/08.") {
		t.Fatalf("LEAK_JOINED %q", j.Body)
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
	if len(w.alerts) != 2 || w.alerts[123].State != entities.AlertFinished || w.active == nil || w.active.ID != 124 {
		t.Fatalf("alertas %d", len(w.alerts))
	}
	if end := w.message("END_NORMAL"); !strings.Contains(end.Body, "queda como la normal") {
		t.Fatalf("END_NORMAL %q", end.Body)
	}
}

// Las horas de FUGA con la alerta sostenida por el movimiento no emiten los
// mensajes horarios de ruido (§7.3.3).
func TestDecide_SustainedByMotionIsSilent(t *testing.T) {
	w := newCWorld(t)
	w.hour(quietLeak(0), mv(0, opened, true, "SALTO", rJump()))
	for n := 1; n < 20; n++ {
		w.leak(calc(n, 0.15, N, N, none, inEpisode(false), belowRun(0)))
		w.leak(noCalc(n + 100))
		w.mov(motionEpisode(n, "NORMAL"))
	}
	for _, m := range w.messages[1:] {
		t.Fatalf("mensaje de ruido con la alerta sostenida por el movimiento: %s", m.Code)
	}
	if !slices.Equal(w.kinds(), []string{"MOV_OPEN:" + notif}) {
		t.Fatalf("mensajes %v", w.kinds())
	}
	_ = messages.MessageCatalogVersion
}
