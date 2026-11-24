package services

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

const pointID = 11556

var hour0 = time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)

func h(n int) time.Time { return hour0.Add(time.Duration(n) * time.Hour) }

func intp(v int) *int { return &v }

// hourIn es una hora de la secuencia que alimenta a Decide.
type hourIn struct {
	notice entities.LevelNotice
}

// calc arma una hora CALCULATED con los campos que usa Alert Management.
func calc(n int, i float64, raw, computed entities.Level, event entities.EpisodeEvent, opts ...func(*entities.LevelNotice)) entities.LevelNotice {
	v := i
	notice := entities.LevelNotice{
		MeasuringPointID: pointID, EventType: entities.EventTypeLeak, EvaluatedAt: h(n),
		Status: entities.StatusCalculated, IndicatorValue: &v,
		RawLevel: raw, ComputedLevel: computed, EpisodeEvent: event,
		InEpisode:          event == entities.EpisodeOpened || event == entities.EpisodeNone && computed.AlertLevel(),
		Thresholds:         entities.Cuts{Amarillo: 0.16, Naranja: 0.25, Rojo: 0.35},
		Exit:               entities.Cuts{Amarillo: 0.14, Naranja: 0.23, Rojo: 0.33},
		ThresholdVersionID: 1, RuleEvaluationID: int64(100 + n),
		EscalationAmarillo: intp(12), EscalationNaranja: intp(6),
	}
	for _, o := range opts {
		o(&notice)
	}
	return notice
}

func runs(amarillo, naranja int) func(*entities.LevelNotice) {
	return func(n *entities.LevelNotice) { n.RunsAmarillo, n.RunsNaranja = amarillo, naranja }
}

func hoursAbove(v int) func(*entities.LevelNotice) {
	return func(n *entities.LevelNotice) { n.HoursAbove = v }
}

func persistence(hours int) func(*entities.LevelNotice) {
	return func(n *entities.LevelNotice) { n.PersistenceHours = hours }
}

func belowRun(v int) func(*entities.LevelNotice) {
	return func(n *entities.LevelNotice) { n.BelowRun = v }
}

func inEpisode(v bool) func(*entities.LevelNotice) {
	return func(n *entities.LevelNotice) { n.InEpisode = v }
}

func lowConfidence(n *entities.LevelNotice) { n.LowConfidence = true }

func noCalc(n int) entities.LevelNotice {
	return entities.NoCalculable(pointID, entities.EventTypeLeak, h(n))
}

// world simula lo que persiste el caso de uso entre horas.
type world struct {
	t        *testing.T
	policy   entities.Policy
	state    entities.ProcessingState
	alerts   map[int64]*entities.Alert
	active   *entities.Alert
	segments map[int64][]entities.Segment
	messages []entities.Message
	nextID   int64
}

func newWorld(t *testing.T) *world {
	return &world{
		t:        t,
		policy:   entities.Policy{ID: 1, Version: 1, EventType: entities.EventTypeLeak, PolicyParams: entities.DefaultPolicyParams()},
		alerts:   map[int64]*entities.Alert{},
		segments: map[int64][]entities.Segment{},
		nextID:   123,
	}
}

func (w *world) openSegment(alertID int64) *entities.Segment {
	segs := w.segments[alertID]
	if len(segs) == 0 || segs[len(segs)-1].EndedAt != nil {
		return nil
	}
	return &segs[len(segs)-1]
}

func (w *world) feed(notices ...entities.LevelNotice) []Decision {
	w.t.Helper()
	out := make([]Decision, 0, len(notices))
	for _, n := range notices {
		in := DecideInput{State: w.state, Notice: n, Policy: w.policy, Device: entities.Device{MeasuringPointID: pointID, PK: "PK34+500"}}
		if w.active != nil {
			a := *w.active
			in.Active = &a
			in.Segment = w.openSegment(a.ID)
		}
		if w.state.LastAlertID != nil {
			last := *w.alerts[*w.state.LastAlertID]
			in.Last = &last
		}
		d, err := Decide(in)
		if err != nil {
			w.t.Fatalf("%v: %v", n.EvaluatedAt, err)
		}
		w.apply(d)
		out = append(out, d)
	}
	return out
}

func (w *world) apply(d Decision) {
	w.state = d.State
	if d.Alert == nil {
		return
	}
	a := *d.Alert
	if d.IsNew {
		a.ID = w.nextID
		a.Code = entities.AlertCode(a.ID)
		w.nextID++
	}
	if d.CloseSegmentAt != nil {
		segs := w.segments[a.ID]
		at := *d.CloseSegmentAt
		segs[len(segs)-1].EndedAt = &at
	}
	if d.NewSegment != nil {
		s := *d.NewSegment
		s.AlertID = a.ID
		s.Seq = len(w.segments[a.ID]) + 1
		w.segments[a.ID] = append(w.segments[a.ID], s)
	}
	for _, m := range d.Messages {
		m.AlertID = a.ID
		w.messages = append(w.messages, m)
	}
	w.alerts[a.ID] = &a
	if a.State == entities.AlertActive {
		w.active = &a
	} else {
		w.active = nil
	}
}

func (w *world) codes() []string {
	out := make([]string, len(w.messages))
	for i, m := range w.messages {
		out[i] = m.Code
	}
	return out
}

func (w *world) codeAt(code string) []time.Time {
	var out []time.Time
	for _, m := range w.messages {
		if m.Code == code {
			out = append(out, m.EvaluatedAt)
		}
	}
	return out
}

func (w *world) notifications() int {
	n := 0
	for _, m := range w.messages {
		if m.Kind == entities.KindNotification {
			n++
		}
	}
	return n
}

func (w *world) levels(alertID int64) []entities.Level {
	var out []entities.Level
	for _, s := range w.segments[alertID] {
		out = append(out, s.Level)
	}
	return out
}

const (
	N = entities.LevelNormal
	A = entities.LevelAmarillo
	O = entities.LevelNaranja
	R = entities.LevelRojo
)

var (
	opened  = entities.EpisodeOpened
	cleared = entities.EpisodeCleared
	none    = entities.EpisodeNone
)

// Escenario 1: una hora sobre 0,16 no crea alerta.
func TestDecide_OneHourAboveDoesNotOpen(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.12, N, N, none),
		calc(1, 0.19, A, A, none, inEpisode(false)),
		calc(2, 0.13, N, N, none),
	)
	if len(w.alerts) != 0 || len(w.messages) != 0 {
		t.Fatalf("alertas = %d, mensajes = %v", len(w.alerts), w.codes())
	}
	if w.state.PendingRunStartedAt != nil {
		t.Fatal("la racha pendiente debe reiniciarse con una hora normal")
	}
}

// Escenario 2 (forma): abre con fecha inicio = primera hora de la racha,
// cuenta regresiva a 3 h de la persistencia, sube por duración y termina.
func TestDecide_OpenCountdownPersistenceUpAndEnd(t *testing.T) {
	w := newWorld(t)
	notices := []entities.LevelNotice{
		calc(0, 0.17, A, A, none, inEpisode(false), runs(1, 0)),
		calc(1, 0.18, A, A, opened, runs(2, 0), hoursAbove(2)),
	}
	for n := 2; n <= 10; n++ {
		notices = append(notices, calc(n, 0.19, A, A, none, runs(n+1, 0), hoursAbove(n+1)))
	}
	notices = append(notices, calc(11, 0.20, A, O, none, runs(12, 0), hoursAbove(12), persistence(12)))
	w.feed(notices...)

	a := w.alerts[123]
	if a == nil || !a.StartedAt.Equal(h(0)) || !a.OpenedAt.Equal(h(1)) {
		t.Fatalf("alerta = %+v; inicio debía ser h0 y creación h1", a)
	}
	if got := w.codeAt(messages.CodeCountdown); len(got) != 1 || !got[0].Equal(h(8)) {
		t.Fatalf("COUNTDOWN en %v; se esperaba h8 (12 − 9 = 3)", got)
	}
	if got := w.codeAt(messages.CodeUpNaranjaPersist); len(got) != 1 || !got[0].Equal(h(11)) {
		t.Fatalf("UP_NARANJA_PERSIST en %v", got)
	}
	if w.notifications() != 2 || a.CurrentLevel != O || a.MaxLevel != O {
		t.Fatalf("notificaciones = %d, alerta = %s/%s", w.notifications(), a.CurrentLevel, a.MaxLevel)
	}
	if !slices.Equal(w.levels(123), []entities.Level{A, O}) {
		t.Fatalf("colores = %v", w.levels(123))
	}

	w.feed(
		calc(12, 0.15, N, N, none, inEpisode(true)),
		calc(13, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(14, 0.12, N, N, cleared, inEpisode(false), belowRun(2)),
	)
	a = w.alerts[123]
	if a.State != entities.AlertFinished || a.EndedAt == nil || !a.EndedAt.Equal(h(14)) {
		t.Fatalf("estado = %s, fin = %v", a.State, a.EndedAt)
	}
	if w.segments[123][1].EndedAt == nil {
		t.Fatal("el color vigente debe cerrarse al finalizar")
	}
	if got := w.codeAt(messages.CodeEndNormal); len(got) != 1 {
		t.Fatalf("END_NORMAL = %v", got)
	}
	if a.HoursCalculated != 15 {
		t.Fatalf("horas = %d, se esperaban 15 (h0..h14)", a.HoursCalculated)
	}
}

// Escenario 3: subida rápida A → N → R con 3 notificaciones.
func TestDecide_FastRise(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.20, A, A, none),
		calc(3, 0.27, O, O, none),
		calc(4, 0.40, R, R, none),
	)
	if !slices.Equal(w.codeAt(messages.CodeUpNaranjaFast), []time.Time{h(3)}) || !slices.Equal(w.codeAt(messages.CodeUpRojo), []time.Time{h(4)}) {
		t.Fatalf("mensajes = %v", w.codes())
	}
	if w.notifications() != 3 {
		t.Fatalf("notificaciones = %d", w.notifications())
	}
	if !w.alerts[123].Blinking() {
		t.Fatal("rojo sin clasificar titila")
	}
}

// Escenario 4: naranja que se estabiliza baja a amarillo a las 6 h, sin notificar.
func TestDecide_DowngradeAfterSixHours(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.27, O, O, opened, hoursAbove(2)),
	)
	for n := 2; n <= 7; n++ {
		w.feed(calc(n, 0.20, A, A, none))
	}
	if got := w.codeAt(messages.CodeDowngradeStart); !slices.Equal(got, []time.Time{h(2)}) {
		t.Fatalf("DOWNGRADE_START en %v", got)
	}
	if got := w.codeAt(messages.CodeDowngrade); !slices.Equal(got, []time.Time{h(7)}) {
		t.Fatalf("DOWNGRADE en %v (mensajes %v)", got, w.codes())
	}
	a := w.alerts[123]
	if a.CurrentLevel != A || a.MaxLevel != O || w.notifications() != 1 {
		t.Fatalf("alerta %s/%s, notificaciones %d", a.CurrentLevel, a.MaxLevel, w.notifications())
	}
	if !slices.Equal(w.levels(123), []entities.Level{O, A}) {
		t.Fatalf("colores = %v", w.levels(123))
	}
}

// Escenario 4 completo: A → N → A son 3 tramos y el máximo queda en Naranja.
func TestDecide_ThreeSegmentsUpAndDown(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.27, O, O, none),
	)
	for n := 3; n <= 8; n++ {
		w.feed(calc(n, 0.20, A, A, none))
	}
	if !slices.Equal(w.levels(123), []entities.Level{A, O, A}) || w.alerts[123].MaxLevel != O {
		t.Fatalf("colores = %v, máximo = %s", w.levels(123), w.alerts[123].MaxLevel)
	}
}

// Escenario 6: rojo sin clasificar no baja; un solo ROJO_HELD.
func TestDecide_RojoUnclassifiedIsHeld(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.40, R, R, opened, hoursAbove(2)),
	)
	for n := 2; n <= 12; n++ {
		w.feed(calc(n, 0.20, A, A, none))
	}
	if got := w.codeAt(messages.CodeRojoHeld); !slices.Equal(got, []time.Time{h(7)}) {
		t.Fatalf("ROJO_HELD en %v", got)
	}
	if a := w.alerts[123]; a.CurrentLevel != R || !a.Blinking() {
		t.Fatalf("alerta %s blinking=%v", a.CurrentLevel, a.Blinking())
	}
}

// Escenario 5: rojo clasificado baja a naranja a las 6 h por debajo.
func TestDecide_RojoClassifiedDowngrades(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.40, R, R, opened, hoursAbove(2)),
	)
	w.active.Management = entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationConfirmed}
	for n := 2; n <= 7; n++ {
		w.feed(calc(n, 0.27, O, O, none))
	}
	a := w.alerts[123]
	if a.CurrentLevel != O || a.Blinking() || len(w.codeAt(messages.CodeRojoHeld)) != 0 {
		t.Fatalf("alerta %s blinking=%v mensajes=%v", a.CurrentLevel, a.Blinking(), w.codes())
	}
}

// Escenario 7: baja a la banda y vuelve; no finaliza.
func TestDecide_AlmostNormalAndBack(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.15, N, N, none, inEpisode(true)),
		calc(3, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(4, 0.19, A, A, none),
	)
	want := []string{messages.CodeOpen, messages.CodeAlmostNormal, messages.CodeOneHourNormal, messages.CodeBackAbnormal}
	if !slices.Equal(w.codes(), want) {
		t.Fatalf("mensajes = %v, se esperaban %v", w.codes(), want)
	}
	if w.alerts[123].State != entities.AlertActive {
		t.Fatal("no finaliza hasta el CLEARED del Rule Engine")
	}
}

// Escenario 8: oscila en el borde ⇒ un OSCILLATING y después silencio de las reglas 5–6.
func TestDecide_OscillationSilencesBandMessages(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
	)
	for n := 2; n <= 9; n++ {
		if n%2 == 0 {
			w.feed(calc(n, 0.15, N, N, none, inEpisode(true)))
		} else {
			w.feed(calc(n, 0.17, A, A, none))
		}
	}
	if got := w.codeAt(messages.CodeOscillating); len(got) != 1 {
		t.Fatalf("OSCILLATING = %v (mensajes %v)", got, w.codes())
	}
	osc := w.codeAt(messages.CodeOscillating)[0]
	for _, m := range w.messages {
		if m.EvaluatedAt.After(osc) && (m.Code == messages.CodeAlmostNormal || m.Code == messages.CodeBackAbnormal) {
			t.Fatalf("%s en %v después de OSCILLATING", m.Code, m.EvaluatedAt)
		}
	}
}

// Escenario 9: hueco de 3 h ⇒ NO_CALC_START / END; la alerta sigue y las cuentas se pausan.
func TestDecide_NoCalcGapPausesCounts(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.27, O, O, opened, hoursAbove(2)),
		calc(2, 0.20, A, A, none),
		noCalc(3), noCalc(4), noCalc(5),
		calc(6, 0.20, A, A, none),
	)
	if got := w.codeAt(messages.CodeNoCalcStart); !slices.Equal(got, []time.Time{h(3)}) {
		t.Fatalf("NO_CALC_START en %v", got)
	}
	if got := w.codeAt(messages.CodeNoCalcEnd); !slices.Equal(got, []time.Time{h(6)}) {
		t.Fatalf("NO_CALC_END en %v", got)
	}
	a := w.alerts[123]
	if a.State != entities.AlertActive || a.CurrentLevel != O || a.HoursNoData != 3 || a.BelowLevelHours != 2 {
		t.Fatalf("alerta = %s %s sin datos %d por debajo %d", a.State, a.CurrentLevel, a.HoursNoData, a.BelowLevelHours)
	}
}

// Escenario 11: Falsa alerta en amarillo; el ruido sube y notifica normalmente.
func TestDecide_FalseAlarmKeepsNotifyingRises(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
	)
	w.active.Management = entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationFalseAlarm}
	w.feed(calc(2, 0.27, O, O, none), calc(3, 0.40, R, R, none))
	if w.notifications() != 3 || w.alerts[123].State != entities.AlertActive || w.alerts[123].Blinking() {
		t.Fatalf("notificaciones %d estado %s", w.notifications(), w.alerts[123].State)
	}
}

// CA-5 (catálogo v7, §7.3.5): con MANTENIMIENTO_PROGRAMADO las subidas
// emiten su UP_* normal como Notificación (ya no existe UP_DURING_INTERVENTION).
func TestDecide_ScheduledWorkStillNotifiesRises(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
	)
	w.active.Management = entities.Management{Status: entities.StatusClassifiedAndLabel,
		Classification: entities.ClassificationFalseAlarm, Label: entities.LabelScheduledWork}
	w.feed(calc(2, 0.27, O, O, none), calc(3, 0.40, R, R, none))
	if got := w.codeAt(messages.CodeUpNaranjaFast); !slices.Equal(got, []time.Time{h(2)}) {
		t.Fatalf("UP_NARANJA_FAST en %v", got)
	}
	if got := w.codeAt(messages.CodeUpRojo); !slices.Equal(got, []time.Time{h(3)}) {
		t.Fatalf("UP_ROJO en %v", got)
	}
	if w.notifications() != 3 { // OPEN + UP_NARANJA_FAST + UP_ROJO
		t.Fatalf("notificaciones = %d", w.notifications())
	}
}

// finishedAt deja la secuencia con una alerta amarilla finalizada en h(3).
func finishedAt(t *testing.T) *world {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(3, 0.12, N, N, cleared, inEpisode(false), belowRun(2)),
	)
	return w
}

// bodyOf devuelve el cuerpo del primer mensaje con ese código.
func (w *world) bodyOf(code string) string {
	for _, m := range w.messages {
		if m.Code == code {
			return m.Body
		}
	}
	return ""
}

// La duración es el tiempo transcurrido (lo que suman los colores), no las
// horas evaluadas: de h0 a h3 son 3 h aunque se evaluaron 4 horas.
func TestDecide_EndDurationIsElapsedTime(t *testing.T) {
	w := finishedAt(t)
	a := w.alerts[123]
	if !a.StartedAt.Equal(h(0)) || !a.EndedAt.Equal(h(3)) {
		t.Fatalf("alerta = %+v", a)
	}
	if body := w.bodyOf(messages.CodeEndNormal); !strings.Contains(body, "Duró 3 h") {
		t.Fatalf("END_NORMAL = %q, se esperaba 3 h (h0 → h3)", body)
	}
	if got := a.DurationHours(*a.EndedAt); got != 3 {
		t.Fatalf("DurationHours = %d", got)
	}
}

// Las horas sin datos con la alerta activa son tiempo transcurrido: cuentan.
func TestDecide_EndDurationIncludesNoDataHours(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		noCalc(2),
		noCalc(3),
		calc(4, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(5, 0.12, N, N, cleared, inEpisode(false), belowRun(2)),
	)
	if body := w.bodyOf(messages.CodeEndNormal); !strings.Contains(body, "Duró 5 h") {
		t.Fatalf("END_NORMAL = %q, se esperaba 5 h (h0 → h5)", body)
	}
}

// La pausa entre el fin y la reactivación no suma: termina en h3, vuelve con
// un color desde h9 y termina en h12 ⇒ 3 + 3 = 6 h.
func TestDecide_EndDurationExcludesReactivationPause(t *testing.T) {
	w := finishedAt(t)
	w.feed(
		calc(9, 0.17, A, A, none, inEpisode(false)),
		calc(10, 0.18, A, A, opened, hoursAbove(2)),
		calc(11, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(12, 0.12, N, N, cleared, inEpisode(false), belowRun(2)),
	)
	var ends []string
	for _, m := range w.messages {
		if m.Code == messages.CodeEndNormal {
			ends = append(ends, m.Body)
		}
	}
	if len(ends) != 2 || !strings.Contains(ends[1], "Duró 6 h") {
		t.Fatalf("END_NORMAL = %q, se esperaba 6 h (3 + 3, sin las 6 h de pausa)", ends)
	}
	var sum time.Duration
	for _, seg := range w.segments[123] {
		sum += seg.EndedAt.Sub(seg.StartedAt)
	}
	if a := w.alerts[123]; a.DurationHours(*a.EndedAt) != int(sum.Hours()) {
		t.Fatalf("DurationHours = %d, los colores suman %v", a.DurationHours(*a.EndedAt), sum)
	}
}

// Escenario 13: vuelve 6 h después sin clasificar ⇒ misma alerta, color nuevo, sin notificar.
func TestDecide_ReactivationWithinWindow(t *testing.T) {
	w := finishedAt(t)
	w.feed(
		calc(9, 0.17, A, A, none, inEpisode(false)),
		calc(10, 0.18, A, A, opened, hoursAbove(2)),
	)
	a := w.alerts[123]
	if len(w.alerts) != 1 || a.State != entities.AlertActive || a.EndedAt != nil || a.ReactivationCount != 1 {
		t.Fatalf("alertas = %d, alerta = %+v", len(w.alerts), a)
	}
	if got := w.codeAt(messages.CodeReactivated); !slices.Equal(got, []time.Time{h(10)}) {
		t.Fatalf("REACTIVATED en %v", got)
	}
	if w.notifications() != 1 {
		t.Fatalf("notificaciones = %d: la reactivación no supera el máximo", w.notifications())
	}
	segs := w.segments[123]
	if len(segs) != 2 || segs[1].OpenedBy != entities.SegmentReactivation || !segs[1].StartedAt.Equal(h(9)) {
		t.Fatalf("colores = %+v", segs)
	}
	// Luego sube a naranja: supera el máximo y notifica.
	w.feed(calc(11, 0.27, O, O, none))
	if w.notifications() != 2 {
		t.Fatalf("notificaciones = %d", w.notifications())
	}
}

// Reactivación que supera el máximo: notifica.
func TestDecide_ReactivationAboveMaxNotifies(t *testing.T) {
	w := finishedAt(t)
	w.feed(
		calc(9, 0.30, O, O, none, inEpisode(false)),
		calc(10, 0.30, O, O, opened, hoursAbove(2)),
	)
	if w.notifications() != 2 || w.alerts[123].CurrentLevel != O || w.alerts[123].MaxLevel != O {
		t.Fatalf("notificaciones %d alerta %s/%s", w.notifications(), w.alerts[123].CurrentLevel, w.alerts[123].MaxLevel)
	}
}

// Escenario 13b: la anterior ya estaba clasificada ⇒ alerta nueva con notificación.
func TestDecide_NewAfterClassified(t *testing.T) {
	w := finishedAt(t)
	w.alerts[123].Management = entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationFalseAlarm}
	w.feed(
		calc(9, 0.17, A, A, none, inEpisode(false)),
		calc(10, 0.18, A, A, opened, hoursAbove(2)),
	)
	if len(w.alerts) != 2 || w.alerts[124] == nil {
		t.Fatalf("alertas = %d", len(w.alerts))
	}
	m := w.messages[len(w.messages)-1]
	if m.Code != messages.CodeNewAfterClassified || m.Kind != entities.KindNotification || !strings.Contains(m.Body, "ALR-000123") {
		t.Fatalf("mensaje = %+v", m)
	}
}

// Escenario 13c: vuelve 30 h después ⇒ alerta nueva.
func TestDecide_NewAfterMergeWindow(t *testing.T) {
	w := finishedAt(t)
	w.feed(
		calc(33, 0.17, A, A, none, inEpisode(false)),
		calc(34, 0.18, A, A, opened, hoursAbove(2)),
	)
	if len(w.alerts) != 2 || w.messages[len(w.messages)-1].Code != messages.CodeOpen {
		t.Fatalf("alertas = %d, mensajes = %v", len(w.alerts), w.codes())
	}
}

// Escenario 15: sensor en ajuste ⇒ mismos códigos con la marca preliminar.
func TestDecide_LowConfidenceMark(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false), lowConfidence),
		calc(1, 0.18, A, A, opened, hoursAbove(2), lowConfidence),
	)
	if m := w.messages[0]; m.Code != messages.CodeOpen || !strings.HasSuffix(m.Body, messages.LowConfidenceMark) {
		t.Fatalf("mensaje = %+v", m)
	}
}

// Resumen cada 6 h sin mensajes.
func TestDecide_SummaryEverySixQuietHours(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
	)
	for n := 2; n <= 7; n++ {
		w.feed(calc(n, 0.18, A, A, none))
	}
	if got := w.codeAt(messages.CodeSummary); !slices.Equal(got, []time.Time{h(7)}) {
		t.Fatalf("SUMMARY en %v (mensajes %v)", got, w.codes())
	}
}

// Tendencia: ±0,02 frente a 3 h antes.
func TestDecide_TrendChange(t *testing.T) {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.17, A, A, opened, hoursAbove(2)),
		calc(2, 0.17, A, A, none),
		calc(3, 0.18, A, A, none),
		calc(4, 0.20, A, A, none), // 0,20 − 0,17 (h1) = 0,03 ⇒ subiendo
	)
	if got := w.codeAt(messages.CodeTrend); !slices.Equal(got, []time.Time{h(4)}) {
		t.Fatalf("TREND en %v (mensajes %v)", got, w.codes())
	}
}

// Un duplicado o una hora vieja no los decide Decide: el caso de uso los filtra.
// Una hora sin alerta y sin cálculo solo actualiza el estado.
func TestDecide_NoCalcWithoutAlertOnlyUpdatesState(t *testing.T) {
	w := newWorld(t)
	w.feed(noCalc(0))
	if w.state.LastStatus != entities.StatusNoCalculable || w.state.LastEvaluatedAt == nil || len(w.messages) != 0 {
		t.Fatalf("estado = %+v", w.state)
	}
}

// Sin estado previo y en mitad de un episodio (módulo habilitado tarde): abre igual.
func TestDecide_OpensWhenStartingInsideAnEpisode(t *testing.T) {
	w := newWorld(t)
	w.feed(calc(0, 0.27, O, O, none, hoursAbove(5)))
	if a := w.alerts[123]; a == nil || a.CurrentLevel != O || !a.StartedAt.Equal(h(-4)) {
		t.Fatalf("alerta = %+v", a)
	}
}

func TestDecide_RejectsNoticeForOtherPoint(t *testing.T) {
	w := newWorld(t)
	active := &entities.Alert{ID: 1, MeasuringPointID: 999, State: entities.AlertActive, CurrentLevel: A, MaxLevel: A}
	_, err := Decide(DecideInput{Active: active, Notice: calc(0, 0.2, A, A, none), Policy: w.policy})
	if err == nil {
		t.Fatal("se esperaba error: la alerta activa es de otro punto")
	}
}

// Los mensajes citan el dato: el índice de la hora y el límite que lo explica
// (cortes 0,16 / 0,25 / 0,35; salidas 0,14 / 0,23 / 0,33).
func TestDecide_MessagesCiteTheData(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T) *world
		code  string
		body  string
	}{
		{"apertura", func(t *testing.T) *world {
			w := newWorld(t)
			w.feed(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)))
			return w
		}, messages.CodeOpen, "El índice de anomalía acústica está por encima de lo normal desde las 05:00 del 24/08. Dato: 0,18 (lo normal es menos de 0,16)."},
		{"apertura naranja", func(t *testing.T) *world {
			w := newWorld(t)
			w.feed(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.27, O, O, opened, hoursAbove(2)))
			return w
		}, messages.CodeOpen, "El índice de anomalía acústica está muy por encima de lo normal desde las 05:00 del 24/08. Dato: 0,27 (Naranja desde 0,25; lo normal es menos de 0,16)."},
		{"subida rápida", fastRise, messages.CodeUpNaranjaFast, "El índice de anomalía acústica está subiendo más rápido de lo normal. Dato: pasó de 0,20 a 0,27 (Naranja desde 0,25)."},
		{"subida a rojo", fastRise, messages.CodeUpRojo, "El índice de anomalía acústica siguió subiendo y ya es demasiado alto. Dato: pasó de 0,27 a 0,40 (Rojo desde 0,35)."},
		{"subida por duración", persistUp, messages.CodeUpNaranjaPersist, "El índice de anomalía acústica lleva 12 h por encima de lo normal. Dato: 0,20 (lo normal es menos de 0,16)."},
		{"cuenta regresiva", persistUp, messages.CodeCountdown, "El índice de anomalía acústica lleva 9 h por encima de 0,16 (ahora en 0,19); faltan 3 h para el cambio de nivel."},
		{"fin", persistUp, messages.CodeEndNormal, "El índice de anomalía acústica volvió a 0,12 (lo normal, menos de 0,16). Duró 14 h; nivel máximo Naranja, con índice de hasta 0,20. Si vuelve en las próximas 24 h y sigue sin clasificar, se reactivará esta misma alerta."},
		{"casi normal", almostNormal, messages.CodeAlmostNormal, "Índice de anomalía acústica en 0,15, ya por debajo de 0,16; la alerta terminará si baja de 0,14 y se mantiene así."},
		{"1 h normal", almostNormal, messages.CodeOneHourNormal, "Índice de anomalía acústica en 0,13, por debajo de 0,14. La alerta sigue en Amarillo hasta completar el cierre."},
		{"vuelve anormal", almostNormal, messages.CodeBackAbnormal, "Índice de anomalía acústica en 0,19; lo normal es menos de 0,16. La alerta sigue en Amarillo."},
		{"empieza a bajar", downgraded, messages.CodeDowngradeStart, "Índice de anomalía acústica en 0,20, por debajo del límite de Naranja (0,25). Si se mantiene así 6 h, la alerta bajará a Amarillo."},
		{"bajó", downgraded, messages.CodeDowngrade, "El índice de anomalía acústica se mantuvo por debajo del límite de Naranja (0,25) durante 6 h; ahora está en 0,20. Máximo alcanzado: Naranja, con índice de hasta 0,27."},
		{"rojo retenido", func(t *testing.T) *world {
			w := newWorld(t)
			w.feed(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.40, R, R, opened, hoursAbove(2)))
			for n := 2; n <= 7; n++ {
				w.feed(calc(n, 0.20, A, A, none))
			}
			return w
		}, messages.CodeRojoHeld, "El índice de anomalía acústica lleva 6 h por debajo del límite de Rojo (0,35) y está en 0,20, pero una alerta roja sin clasificar no baja de nivel."},
		{"sin datos", noCalcGap, messages.CodeNoCalcStart, "No se puede evaluar desde las 08:00 del 24/08; la alerta sigue en Naranja. Último índice de anomalía acústica: 0,20."},
		{"vuelven los datos", noCalcGap, messages.CodeNoCalcEnd, "Índice de anomalía acústica en 0,20; la alerta sigue en Naranja."},
		{"reactivación", func(t *testing.T) *world {
			w := finishedAt(t)
			w.feed(calc(9, 0.17, A, A, none, inEpisode(false)), calc(10, 0.18, A, A, opened, hoursAbove(2)))
			return w
		}, messages.CodeReactivated, "El índice de anomalía acústica volvió a subir 7 h después de normalizarse. Dato: 0,18 (lo normal es menos de 0,16)."},
		{"nueva tras clasificada", func(t *testing.T) *world {
			w := finishedAt(t)
			w.alerts[123].Management = entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationFalseAlarm}
			w.feed(calc(9, 0.17, A, A, none, inEpisode(false)), calc(10, 0.18, A, A, opened, hoursAbove(2)))
			return w
		}, messages.CodeNewAfterClassified, "El índice de anomalía acústica otra vez está fuera de lo normal; la alerta anterior (ALR-000123) se clasificó como Falsa alerta. Dato: 0,18 (lo normal es menos de 0,16)."},
		{"resumen", func(t *testing.T) *world {
			w := newWorld(t)
			w.feed(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)))
			for n := 2; n <= 7; n++ {
				w.feed(calc(n, 0.18, A, A, none))
			}
			return w
		}, messages.CodeSummary, "Sigue en Amarillo, lleva 7 h, ruido estable. Índice de anomalía acústica en 0,18 (lo normal, menos de 0,16); el máximo de la alerta fue 0,18."},
		{"tendencia", func(t *testing.T) *world {
			w := newWorld(t)
			w.feed(
				calc(0, 0.17, A, A, none, inEpisode(false)),
				calc(1, 0.17, A, A, opened, hoursAbove(2)),
				calc(2, 0.17, A, A, none),
				calc(3, 0.18, A, A, none),
				calc(4, 0.20, A, A, none),
			)
			return w
		}, messages.CodeTrend, "El índice de anomalía acústica pasó de 0,17 a 0,20 en 3 h. La alerta sigue en Amarillo y lleva 4 h."},
		{"oscila", func(t *testing.T) *world {
			w := newWorld(t)
			w.feed(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.18, A, A, opened, hoursAbove(2)))
			for n := 2; n <= 5; n++ {
				if n%2 == 0 {
					w.feed(calc(n, 0.15, N, N, none, inEpisode(true)))
				} else {
					w.feed(calc(n, 0.17, A, A, none))
				}
			}
			return w
		}, messages.CodeOscillating, "El índice de anomalía acústica cruzó 4 veces el límite normal (0,16) desde las 07:00 del 24/08; ahora está en 0,17. No se informará cada cambio."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := c.build(t)
			if got := w.bodyOf(c.code); got != c.body {
				t.Fatalf("%s:\n got %q\nwant %q", c.code, got, c.body)
			}
		})
	}
}

// El máximo del índice queda en la alerta (lo citan el fin, la bajada y el resumen).
func TestDecide_TracksMaxIndicatorValue(t *testing.T) {
	w := fastRise(t)
	w.feed(calc(5, 0.30, O, O, none))
	if got := w.alerts[123].MaxIndicatorValue; got != 0.40 {
		t.Fatalf("máximo = %v, se esperaba 0,40", got)
	}
	r := finishedAt(t)
	r.feed(calc(9, 0.17, A, A, none, inEpisode(false)), calc(10, 0.17, A, A, opened, hoursAbove(2)))
	if got := r.alerts[123].MaxIndicatorValue; got != 0.18 {
		t.Fatalf("máximo tras reactivar = %v, se esperaba 0,18 (el de antes del fin)", got)
	}
}

func fastRise(t *testing.T) *world {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.20, A, A, none),
		calc(3, 0.27, O, O, none),
		calc(4, 0.40, R, R, none),
	)
	return w
}

func persistUp(t *testing.T) *world {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false), runs(1, 0)),
		calc(1, 0.18, A, A, opened, runs(2, 0), hoursAbove(2)),
	)
	for n := 2; n <= 10; n++ {
		w.feed(calc(n, 0.19, A, A, none, runs(n+1, 0), hoursAbove(n+1)))
	}
	w.feed(
		calc(11, 0.20, A, O, none, runs(12, 0), hoursAbove(12), persistence(12)),
		calc(12, 0.15, N, N, none, inEpisode(true)),
		calc(13, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(14, 0.12, N, N, cleared, inEpisode(false), belowRun(2)),
	)
	return w
}

func almostNormal(t *testing.T) *world {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.18, A, A, opened, hoursAbove(2)),
		calc(2, 0.15, N, N, none, inEpisode(true)),
		calc(3, 0.13, N, N, none, inEpisode(true), belowRun(1)),
		calc(4, 0.19, A, A, none),
	)
	return w
}

func downgraded(t *testing.T) *world {
	w := newWorld(t)
	w.feed(calc(0, 0.17, A, A, none, inEpisode(false)), calc(1, 0.27, O, O, opened, hoursAbove(2)))
	for n := 2; n <= 7; n++ {
		w.feed(calc(n, 0.20, A, A, none))
	}
	return w
}

func noCalcGap(t *testing.T) *world {
	w := newWorld(t)
	w.feed(
		calc(0, 0.17, A, A, none, inEpisode(false)),
		calc(1, 0.27, O, O, opened, hoursAbove(2)),
		calc(2, 0.20, A, A, none),
		noCalc(3), noCalc(4), noCalc(5),
		calc(6, 0.20, A, A, none),
	)
	return w
}
