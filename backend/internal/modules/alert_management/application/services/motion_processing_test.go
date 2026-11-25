package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

type fakeMotions struct {
	rows map[int64][]entities.MotionNotice
}

func (f fakeMotions) ListPoints(context.Context) ([]int64, error) {
	var out []int64
	for p := range f.rows {
		out = append(out, p)
	}
	return out, nil
}

func (f fakeMotions) ListAfter(_ context.Context, pointID int64, after, to time.Time, limit int) ([]entities.MotionNotice, error) {
	var out []entities.MotionNotice
	for _, n := range f.rows[pointID] {
		if (after.IsZero() || n.EvaluatedAt.After(after)) && (to.IsZero() || n.EvaluatedAt.Before(to)) && n.AlertsEnabled {
			out = append(out, n)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func motionHour(n int, ev entities.EpisodeEvent, inEpisode bool, reasons ...entities.MotionReason) entities.MotionNotice {
	return entities.MotionNotice{
		MotionEvaluationID: int64(5000 + n), MeasuringPointID: pointA, EvaluatedAt: h(n), Status: entities.StatusCalculated,
		MotionState: "NORMAL", TempState: "TEMP_VARIABLE", InEpisode: inEpisode, EpisodeEvent: ev,
		Reasons: reasons, AlertsEnabled: true,
	}
}

func abrupt() entities.MotionReason {
	return entities.MotionReason{Code: entities.MotionAbrupt, Axis: "roll", Delta: 0.04, Hours: 1}
}

func (f *fixture) motion(t *testing.T, source entities.Source, notices ...entities.MotionNotice) []services.ProcessResult {
	t.Helper()
	var out []services.ProcessResult
	for _, n := range notices {
		if source == entities.SourceLive {
			f.now = n.EvaluatedAt.Add(40 * time.Minute)
		}
		r, err := f.svc.ProcessMotion(context.Background(), n, source)
		if err != nil {
			t.Fatalf("%v: %v", n.EvaluatedAt, err)
		}
		out = append(out, r)
	}
	return out
}

// CA-7: en modo sombra la hora no abre ni toca alertas.
func TestProcessMotion_ShadowModeIsIgnored(t *testing.T) {
	f := newFixture()
	n := motionHour(0, entities.EpisodeOpened, true, abrupt())
	n.AlertsEnabled = false
	r := f.motion(t, entities.SourceLive, n)
	if r[0].Outcome != services.ProcessIgnored || len(f.store.Alerts()) != 0 || len(f.pub.published) != 0 {
		t.Fatalf("modo sombra: %+v, alertas %d, avisos %d", r[0], len(f.store.Alerts()), len(f.pub.published))
	}
}

// CA-8: un evento duplicado no crea filas ni mensajes; el aviso sale con event_type MOVIMIENTO.
func TestProcessMotion_DuplicateDoesNothing(t *testing.T) {
	f := newFixture()
	open := motionHour(0, entities.EpisodeOpened, true, abrupt())
	f.motion(t, entities.SourceLive, open)
	again := f.motion(t, entities.SourceLive, open)

	if again[0].Outcome != services.ProcessDuplicate || len(f.store.Alerts()) != 1 || len(f.store.Messages()) != 1 || len(f.pub.published) != 1 {
		t.Fatalf("duplicado: %+v, alertas %d, mensajes %d, avisos %d", again[0], len(f.store.Alerts()), len(f.store.Messages()), len(f.pub.published))
	}
	if e := f.pub.published[0]; e.EventType != "MOVIMIENTO" || e.Code != messages.CodeMovOpen {
		t.Fatalf("aviso %+v", e)
	}
}

// Un evento en vivo después de un hueco completa las horas desde motion_evaluations.
func TestProcessMotion_FillsMissingHoursFromSource(t *testing.T) {
	f := newFixture()
	rows := []entities.MotionNotice{motionHour(0, entities.EpisodeNone, false), motionHour(1, entities.EpisodeOpened, true, abrupt()),
		motionHour(2, entities.EpisodeNone, true)}
	f.svc.WithMotionSource(fakeMotions{rows: map[int64][]entities.MotionNotice{pointA: rows}})
	f.motion(t, entities.SourceLive, rows[0], rows[2])
	if len(f.store.Alerts()) != 1 || f.store.Messages()[0].Code != messages.CodeMovOpen {
		t.Fatalf("la hora perdida (OPENED) no se completó: %d alertas", len(f.store.Alerts()))
	}
}

// §7.2: en el backfill, en cada hora primero FUGA y después MOVIMIENTO. Si
// se solapan son DOS alertas: la de FUGA abre con OPEN y la de MOVIMIENTO con
// MOV_OPEN (no se combinan).
func TestBackfill_InterleavesFlowsLeakFirst(t *testing.T) {
	f := newFixture()
	f.now = h(500)
	levels := fakeLevels{rows: map[int64][]entities.LevelNotice{pointA: opening()}}
	motions := fakeMotions{rows: map[int64][]entities.MotionNotice{pointA: {
		motionHour(0, entities.EpisodeNone, false), motionHour(1, entities.EpisodeOpened, true, abrupt()),
	}}}
	result, err := newReplay(f, levels, "development").WithMotionSource(motions).Backfill(context.Background(), services.BackfillCommand{})
	if err != nil {
		t.Fatal(err)
	}
	msgs := f.store.Messages()
	if len(msgs) != 2 || msgs[0].Code != messages.CodeOpen || msgs[1].Code != messages.CodeMovOpen {
		t.Fatalf("mensajes %v", codesOf(msgs))
	}
	alerts := f.store.Alerts()
	if len(alerts) != 2 || alerts[0].EventType != entities.EventTypeLeak || alerts[1].EventType != entities.EventTypeMovement ||
		alerts[0].State != entities.AlertActive || alerts[1].State != entities.AlertActive ||
		msgs[0].AlertID != alerts[0].ID || msgs[1].AlertID != alerts[1].ID || result.MotionHours != 2 || len(f.pub.published) != 0 {
		t.Fatalf("alertas %+v, resultado %+v", alerts, result.PointReplay)
	}
}

// Con las dos alertas activas, un TEMP_* sale en las dos (mismo tipo de
// mensaje); el aviso de la copia en FUGA lleva event_type FUGA y su alerta.
func TestProcessMotion_TemperatureReachesActiveLeakAlert(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	leak := f.store.Alerts()[0]
	f.motion(t, entities.SourceLive, motionHour(10, entities.EpisodeOpened, true, abrupt()))
	from, rate := h(9), 0.02
	stab := motionHour(11, entities.EpisodeNone, true, entities.MotionReason{Code: entities.MotionTempStabilized, TempRateMax6h: &rate, TempStableFrom: &from})
	stab.TempState = entities.TempEstable
	before := len(f.pub.published)
	f.motion(t, entities.SourceLive, stab)

	var onLeak, onMotion int
	for _, m := range f.store.Messages() {
		if m.Code != messages.CodeTempStabilized {
			continue
		}
		if m.AlertID == leak.ID {
			onLeak++
		} else {
			onMotion++
		}
		if m.Kind != entities.KindNotification {
			t.Fatalf("TEMP_STABILIZED con kind %s", m.Kind)
		}
	}
	if onLeak != 1 || onMotion != 1 {
		t.Fatalf("TEMP_STABILIZED en FUGA %d, en MOVIMIENTO %d", onLeak, onMotion)
	}
	got := f.pub.published[before:]
	if len(got) != 2 || got[0].EventType != "MOVIMIENTO" || got[1].EventType != "FUGA" || got[1].AlertID != leak.ID {
		t.Fatalf("avisos %+v", got)
	}
	if a, _ := f.store.Alert(leak.ID); a.TempState != entities.TempEstable || a.TempStableFrom == nil || a.MotionState != "" || a.CurrentLevel != leak.CurrentLevel {
		t.Fatalf("alerta de FUGA %+v", a)
	}
}

func codesOf(ms []entities.Message) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Code
	}
	return out
}

// CA-10 y CA-9: sin -drop-management el rebuild de un punto con gestión
// aborta sin escribir; con él, borra la gestión y reconstruye.
func TestBackfill_RebuildGuardForManagement(t *testing.T) {
	f := newFixture()
	f.now = h(500)
	levels := fakeLevels{rows: map[int64][]entities.LevelNotice{pointA: leakRows()}}
	replay := newReplay(f, levels, "development")
	if _, err := replay.Backfill(context.Background(), services.BackfillCommand{}); err != nil {
		t.Fatal(err)
	}
	id := f.store.Alerts()[0].ID
	mgmt := services.NewManagementService(f.store, ports.ClockFunc(func() time.Time { return f.now }), testLogger())
	if _, err := mgmt.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationFalseAlarm,
		Observation: "prueba", ActionOrigin: web()}); err != nil {
		t.Fatal(err)
	}
	before := len(f.store.Messages())

	_, err := replay.Backfill(context.Background(), services.BackfillCommand{Rebuild: true})
	if !errors.Is(err, domain.ErrManagementPresent) {
		t.Fatalf("err = %v, se esperaba ErrManagementPresent", err)
	}
	if a, _ := f.store.Alert(id); !a.Management.Classified() || len(f.store.Messages()) != before || len(f.store.Observations()) != 1 {
		t.Fatal("sin -drop-management no debía borrar nada")
	}
	if _, err := replay.Backfill(context.Background(), services.BackfillCommand{DropManagement: true}); !errors.Is(err, domain.ErrDropManagementRequiresRebuild) {
		t.Fatalf("-drop-management sin -rebuild: %v", err)
	}

	result, err := replay.Backfill(context.Background(), services.BackfillCommand{Rebuild: true, DropManagement: true})
	if err != nil {
		t.Fatal(err)
	}
	c := result.Points[0].Reset
	if c.Alerts != 1 || c.Classified != 1 || c.Observations != 1 || c.Messages != int64(before) {
		t.Fatalf("conteo %+v", c)
	}
	alerts := f.store.Alerts()
	if len(alerts) != 1 || alerts[0].Management.Classified() || len(f.store.Observations()) != 0 {
		t.Fatalf("tras el rebuild queda gestión: %+v", alerts)
	}
}

// CA-3: clasificar a las 14:23:10 crea CLASSIFIED con evaluated_at = 14:23:10
// en el color vigente; etiquetar crea LABELED con su hora; un conflicto o un
// reenvío no crean mensajes.
func TestManagement_MessagesAtTheirTime(t *testing.T) {
	f, _, id := managementFixture(t)
	clock := time.Date(2026, 8, 24, 19, 23, 10, 500_000_000, time.UTC) // 14:23:10,5 en Colombia
	svc := services.NewManagementService(f.store, ports.ClockFunc(func() time.Time { return clock }), testLogger())
	recorded := clock.Add(-30 * time.Minute)
	origin := mobile(clientID)
	origin.RecordedAt = &recorded

	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationFalseAlarm,
		Observation: "maniobra", ActionOrigin: origin}); err != nil {
		t.Fatal(err)
	}
	msgs := f.store.Messages()
	m := msgs[len(msgs)-1]
	want := time.Date(2026, 8, 24, 19, 23, 10, 0, time.UTC)
	segments := f.store.Segments(id)
	if m.Code != messages.CodeClassified || !m.EvaluatedAt.Equal(want) || m.SegmentID != segments[len(segments)-1].ID {
		t.Fatalf("CLASSIFIED %+v", m)
	}
	if m.Body != "Registrada desde la app el 24/08 a las 14:23. Hecha en el celular a las 13:53. Observación: «maniobra»" ||
		m.Params["classification"] != "FALSA_ALERTA" || m.Params["origin"] != "APP_MOVIL" || m.Params["device_identifier"] == "" {
		t.Fatalf("CLASSIFIED %q %v", m.Body, m.Params)
	}
	if a, _ := f.store.Alert(id); !a.Management.ClassifiedAt.Equal(want) {
		t.Fatalf("classified_at = %v", a.Management.ClassifiedAt)
	}

	count := len(f.store.Messages())
	if r, _ := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: web()}); r.Outcome != services.OutcomeConflict {
		t.Fatalf("conflicto: %+v", r)
	}
	if r, _ := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationFalseAlarm, ActionOrigin: mobile(clientID)}); r.Outcome != services.OutcomeAlreadyReceived {
		t.Fatalf("reenvío: %+v", r)
	}
	if len(f.store.Messages()) != count {
		t.Fatal("un conflicto o un reenvío crearon mensajes")
	}

	clock = clock.Add(2 * time.Minute)
	if _, err := svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelScheduledWork, ActionOrigin: web()}); err != nil {
		t.Fatal(err)
	}
	msgs = f.store.Messages()
	l := msgs[len(msgs)-1]
	if l.Code != messages.CodeLabeled || !l.EvaluatedAt.Equal(want.Add(2*time.Minute)) || l.Title != "Etiqueta de la alerta: Mantenimiento programado" ||
		l.Body != "Registrada desde la web el 24/08 a las 14:25." {
		t.Fatalf("LABELED %+v", l)
	}
}

// CA-2: una sola ACTIVA por (punto, evento); FUGA y MOVIMIENTO pueden estar
// activas a la vez porque son alertas distintas.
func TestStore_OneActivePerPointAndEvent(t *testing.T) {
	f := newFixture()
	insert := func(e entities.EventType) error {
		return f.store.Transaction(context.Background(), func(tx ports.AlertTx) error {
			_, err := tx.InsertAlert(context.Background(), entities.Alert{MeasuringPointID: pointA, EventType: e, State: entities.AlertActive})
			return err
		})
	}
	if err := insert(entities.EventTypeLeak); err != nil {
		t.Fatal(err)
	}
	if err := insert(entities.EventTypeMovement); err != nil {
		t.Fatalf("FUGA y MOVIMIENTO activas a la vez: %v", err)
	}
	for _, e := range []entities.EventType{entities.EventTypeLeak, entities.EventTypeMovement} {
		if err := insert(e); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("segunda %s activa: err = %v, se esperaba ErrConflict", e, err)
		}
	}
}
