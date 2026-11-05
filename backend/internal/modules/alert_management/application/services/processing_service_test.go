package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const pointA int64 = 11556

var hour0 = time.Date(2026, 8, 24, 16, 0, 0, 0, time.UTC)

func h(n int) time.Time { return hour0.Add(time.Duration(n) * time.Hour) }

func testLogger() *logger.Logger { return logger.New("AlertManagementTest", "Error") }

type fakeDevices map[int64]entities.Device

func (f fakeDevices) Get(_ context.Context, pointID int64) (*entities.Device, error) {
	d, ok := f[pointID]
	if !ok {
		return nil, nil
	}
	return &d, nil
}

// fakePublisher guarda los avisos publicados (o falla si err != nil).
type fakePublisher struct {
	published []contract.AlertNotification
	err       error
}

func (p *fakePublisher) PublishAlertNotification(_ context.Context, n contract.AlertNotification) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, n)
	return nil
}

// fakeChangePublisher guarda las fotos publicadas (o falla si err != nil).
type fakeChangePublisher struct {
	published []contract.AlertChanged
	err       error
}

func (p *fakeChangePublisher) PublishAlertChanged(_ context.Context, c contract.AlertChanged) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, c)
	return nil
}

func (p *fakeChangePublisher) changes() []string {
	var out []string
	for _, c := range p.published {
		out = append(out, c.Change)
	}
	return out
}

type fixture struct {
	store    *memory.AlertStore
	policies *memory.PolicyStore
	pub      *fakePublisher
	changes  *fakeChangePublisher
	now      time.Time
	svc      *services.ProcessingService
}

func newFixture() *fixture {
	f := &fixture{store: memory.NewAlertStore(), policies: memory.NewPolicyStore(), pub: &fakePublisher{},
		changes: &fakeChangePublisher{}, now: h(0).Add(35 * time.Minute)}
	f.svc = services.NewProcessingService(f.store, f.policies,
		fakeDevices{pointA: {MeasuringPointID: pointA, PK: "PK34+500"}}, f.pub,
		ports.ClockFunc(func() time.Time { return f.now }), testLogger()).
		WithChangePublisher(f.changes)
	return f
}

func notice(n int, i float64, raw, computed entities.Level, event entities.EpisodeEvent, hoursAbove int) entities.LevelNotice {
	v := i
	return entities.LevelNotice{
		MeasuringPointID: pointA, EventType: entities.EventTypeLeak, EvaluatedAt: h(n),
		Status: entities.StatusCalculated, IndicatorValue: &v,
		RawLevel: raw, ComputedLevel: computed, EpisodeEvent: event,
		InEpisode:  event == entities.EpisodeOpened || (event == entities.EpisodeNone && hoursAbove > 0),
		HoursAbove: hoursAbove, ThresholdVersionID: 1, RuleEvaluationID: int64(1000 + n),
		Thresholds: entities.Cuts{Amarillo: 0.16, Naranja: 0.25, Rojo: 0.35},
		Exit:       entities.Cuts{Amarillo: 0.14, Naranja: 0.23, Rojo: 0.33},
	}
}

// process alimenta las horas "en vivo": el reloj va 35 min después de cada una.
func (f *fixture) process(t *testing.T, source entities.Source, notices ...entities.LevelNotice) []services.ProcessResult {
	t.Helper()
	var out []services.ProcessResult
	for _, n := range notices {
		if source == entities.SourceLive {
			f.now = n.EvaluatedAt.Add(35 * time.Minute)
		}
		r, err := f.svc.Process(context.Background(), n, source)
		if err != nil {
			t.Fatalf("%v: %v", n.EvaluatedAt, err)
		}
		out = append(out, r)
	}
	return out
}

func opening() []entities.LevelNotice {
	return []entities.LevelNotice{
		notice(0, 0.17, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 0),
		notice(1, 0.18, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeOpened, 2),
	}
}

func TestProcess_LiveOpenPublishesTheNotification(t *testing.T) {
	f := newFixture()
	r := f.process(t, entities.SourceLive, opening()...)

	alerts := f.store.Alerts()
	if len(alerts) != 1 || alerts[0].Code != "ALR-000123" || alerts[0].Source != entities.SourceLive || !alerts[0].StartedAt.Equal(h(0)) {
		t.Fatalf("alertas = %+v", alerts)
	}
	msgs := f.store.Messages()
	if len(msgs) != 1 || msgs[0].Code != messages.CodeOpen || msgs[0].Kind != entities.KindNotification || msgs[0].SegmentID == 0 {
		t.Fatalf("mensajes = %+v", msgs)
	}
	if len(f.pub.published) != 1 || r[1].Notifications != 1 {
		t.Fatalf("publicados = %+v", f.pub.published)
	}
	n := f.pub.published[0]
	if n.MessageID != msgs[0].ID || n.AlertID != alerts[0].ID || n.AlertCode != "ALR-000123" || n.Code != messages.CodeOpen ||
		n.Level != "AMARILLO" || n.MaxLevel != "AMARILLO" || n.AlertState != "ACTIVA" || n.MeasuringPointID != pointA ||
		n.EventType != "FUGA" || !n.StartedAt.Equal(h(0)) || n.Classification != "" ||
		!n.EvaluatedAt.Equal(h(1)) || n.Title != msgs[0].Title || n.Body != msgs[0].Body ||
		n.Source != "LIVE" || n.Late || n.SchemaVersion != contract.SchemaVersion || !n.OccurredAt.Equal(f.now) {
		t.Fatalf("aviso = %+v", n)
	}
	if segs := f.store.Segments(alerts[0].ID); len(segs) != 1 || segs[0].Seq != 1 || segs[0].EndedAt != nil {
		t.Fatalf("colores = %+v", segs)
	}
}

func TestProcess_DuplicateOrOldHourIsIgnored(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	r := f.process(t, entities.SourceLive, opening()[1], opening()[0])
	if r[0].Outcome != services.ProcessDuplicate || r[1].Outcome != services.ProcessDuplicate {
		t.Fatalf("resultados = %+v", r)
	}
	if len(f.store.Messages()) != 1 || len(f.pub.published) != 1 {
		t.Fatal("un duplicado no agrega mensajes ni avisos")
	}
}

// Escenario 9 por catch-up: las horas sin cálculo no tienen fila; el salto de
// evaluated_at se procesa como horas NO_CALCULABLE.
func TestProcess_GapInEvaluatedAtIsProcessedAsNoCalculable(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	r := f.process(t, entities.SourceLive, notice(5, 0.19, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 5))
	if r[0].Hours != 4 {
		t.Fatalf("horas procesadas = %d, se esperaban 3 sin cálculo + 1", r[0].Hours)
	}
	a := f.store.Alerts()[0]
	if a.HoursNoData != 3 || a.State != entities.AlertActive {
		t.Fatalf("alerta = %+v", a)
	}
	var codes []string
	for _, m := range f.store.Messages() {
		codes = append(codes, m.Code+"@"+m.EvaluatedAt.Format("15"))
	}
	want := []string{messages.CodeOpen + "@17", messages.CodeNoCalcStart + "@18", messages.CodeNoCalcEnd + "@21"}
	if len(codes) != len(want) || codes[1] != want[1] || codes[2] != want[2] {
		t.Fatalf("mensajes = %v, se esperaban %v", codes, want)
	}
}

func TestProcess_BackfillNeverNotifies(t *testing.T) {
	f := newFixture()
	f.now = h(200)
	f.process(t, entities.SourceBackfill, opening()...)
	if a := f.store.Alerts()[0]; a.Source != entities.SourceBackfill {
		t.Fatalf("source = %s", a.Source)
	}
	if m := f.store.Messages()[0]; m.Kind != entities.KindNotification {
		t.Fatalf("mensaje = %+v: en el backfill queda registrado como NOTIFICACION", m)
	}
	if len(f.pub.published) != 0 {
		t.Fatal("el backfill nunca publica avisos")
	}
}

// Escenario 20: una hora que llega por el catch-up con más de
// late_notification_max_hours de atraso se publica marcada Late (el módulo de
// notificaciones decide qué canal omite).
func TestProcess_LateCatchUpPublishesMarkedLate(t *testing.T) {
	f := newFixture()
	f.now = h(1).Add(5 * time.Hour)
	f.process(t, entities.SourceCatchUp, opening()...)
	if len(f.pub.published) != 1 || !f.pub.published[0].Late || f.pub.published[0].Source != "CATCH_UP" {
		t.Fatalf("publicados = %+v", f.pub.published)
	}
}

func TestProcess_OnTimeCatchUpIsNotLate(t *testing.T) {
	f := newFixture()
	f.now = h(1).Add(90 * time.Minute)
	f.process(t, entities.SourceCatchUp, opening()...)
	if len(f.pub.published) != 1 || f.pub.published[0].Late {
		t.Fatalf("publicados = %+v", f.pub.published)
	}
}

func TestProcess_StoreFailureSavesAndPublishesNothing(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()[0])
	f.store.FailNextTransaction(errors.New("bd caída"))
	if _, err := f.svc.Process(context.Background(), opening()[1], entities.SourceLive); err == nil {
		t.Fatal("se esperaba error")
	}
	if len(f.store.Alerts()) != 0 || len(f.pub.published) != 0 {
		t.Fatal("sin commit no hay alerta ni aviso")
	}
}

// El aviso se publica después del commit: si falla, la alerta queda guardada.
func TestProcess_PublishFailureKeepsTheAlert(t *testing.T) {
	f := newFixture()
	f.pub.err = errors.New("bus caído")
	r := f.process(t, entities.SourceLive, opening()...)
	if len(f.store.Alerts()) != 1 || len(f.store.Messages()) != 1 || r[1].Outcome != services.ProcessDone {
		t.Fatalf("alertas = %d, resultado = %+v", len(f.store.Alerts()), r[1])
	}
}

// La gestión escrita entre dos horas no la pisa el procesamiento.
func TestProcess_DoesNotOverwriteManagement(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	id := f.store.Alerts()[0].ID
	f.store.SetManagement(id, entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationConfirmed})
	f.process(t, entities.SourceLive, notice(2, 0.27, entities.LevelNaranja, entities.LevelNaranja, entities.EpisodeNone, 3))
	a, _ := f.store.Alert(id)
	if a.Management.Classification != entities.ClassificationConfirmed || a.CurrentLevel != entities.LevelNaranja {
		t.Fatalf("alerta = %+v", a)
	}
	if segs := f.store.Segments(id); len(segs) != 2 || segs[0].EndedAt == nil || segs[1].Seq != 2 {
		t.Fatalf("colores = %+v", segs)
	}
}

// Una falsa alerta que sube de nivel sigue publicando su NOTIFICACION, ahora
// con la clasificación: el módulo de notificaciones decide qué canal la omite.
func TestProcess_NotificationCarriesTheClassification(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	id := f.store.Alerts()[0].ID
	f.store.SetManagement(id, entities.Management{Status: entities.StatusClassifiedNoLabel, Classification: entities.ClassificationFalseAlarm})
	f.process(t, entities.SourceLive, notice(2, 0.27, entities.LevelNaranja, entities.LevelNaranja, entities.EpisodeNone, 3))

	if len(f.pub.published) != 2 {
		t.Fatalf("publicados = %+v", f.pub.published)
	}
	if n := f.pub.published[1]; n.Classification != "FALSA_ALERTA" || n.Level != "NARANJA" || !n.StartedAt.Equal(h(0)) {
		t.Fatalf("aviso = %+v", n)
	}
}

// Escenario 13 de punta a punta: finaliza y se reactiva la misma alerta.
func TestProcess_ReactivationThroughTheStore(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	end := notice(2, 0.12, entities.LevelNormal, entities.LevelNormal, entities.EpisodeCleared, 0)
	end.BelowRun = 2
	f.process(t, entities.SourceLive, end)
	f.process(t, entities.SourceLive,
		notice(8, 0.17, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 0),
		notice(9, 0.18, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeOpened, 2),
	)
	alerts := f.store.Alerts()
	if len(alerts) != 1 || alerts[0].State != entities.AlertActive || alerts[0].ReactivationCount != 1 {
		t.Fatalf("alertas = %+v", alerts)
	}
	segs := f.store.Segments(alerts[0].ID)
	if len(segs) != 2 || segs[1].OpenedBy != entities.SegmentReactivation || segs[0].EndedAt == nil || !segs[0].EndedAt.Equal(h(2)) {
		t.Fatalf("colores = %+v", segs)
	}
	if len(f.pub.published) != 1 {
		t.Fatalf("publicados = %d: la reactivación sin superar el máximo no notifica", len(f.pub.published))
	}
}

func TestProcess_InvalidPoint(t *testing.T) {
	f := newFixture()
	n := opening()[0]
	n.MeasuringPointID = 0
	if _, err := f.svc.Process(context.Background(), n, entities.SourceLive); err == nil {
		t.Fatal("se esperaba error")
	}
}
