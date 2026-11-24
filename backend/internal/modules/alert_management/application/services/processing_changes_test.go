package services_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

func TestProcess_OpenPublishesTheSnapshot(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)

	if got := f.changes.changes(); !slices.Equal(got, []string{contract.ChangeOpened}) {
		t.Fatalf("cambios = %v", got)
	}
	a := f.store.Alerts()[0]
	c := f.changes.published[0]
	if c.AlertID != a.ID || c.AlertCode != a.Code || c.MeasuringPointID != pointA || c.EventType != "FUGA" ||
		c.State != "ACTIVA" || c.CurrentLevel != "AMARILLO" || c.MaxLevel != "AMARILLO" || c.Blinking ||
		c.ManagementStatus != "SIN_CLASIFICAR" || c.Classification != "" || !c.StartedAt.Equal(h(0)) ||
		!c.EvaluatedAt.Equal(h(1)) || !c.OccurredAt.Equal(f.now) || c.SchemaVersion != contract.SchemaVersion ||
		c.EndedAt != nil {
		t.Fatalf("foto = %+v", c)
	}
}

func TestProcess_RaiseToRojoPublishesBlinkingSnapshot(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	f.process(t, entities.SourceLive, notice(2, 0.40, entities.LevelRojo, entities.LevelRojo, entities.EpisodeNone, 3))

	if got := f.changes.changes(); !slices.Equal(got, []string{contract.ChangeOpened, contract.ChangeLevelRaised}) {
		t.Fatalf("cambios = %v", got)
	}
	if c := f.changes.published[1]; c.CurrentLevel != "ROJO" || c.MaxLevel != "ROJO" || !c.Blinking || !c.EvaluatedAt.Equal(h(2)) {
		t.Fatalf("foto = %+v", c)
	}
}

// Un hueco de 3 horas: solo la primera hora sin cálculo y el regreso de los
// datos son cambios visibles; las horas intermedias no publican nada.
func TestProcess_NoDataStartAndEndArePublishedOnce(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	f.process(t, entities.SourceLive, notice(5, 0.19, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 5))

	want := []string{contract.ChangeOpened, contract.ChangeNoDataStarted, contract.ChangeNoDataEnded}
	if got := f.changes.changes(); !slices.Equal(got, want) {
		t.Fatalf("cambios = %v, se esperaban %v", got, want)
	}
	if !f.changes.published[1].EvaluatedAt.Equal(h(2)) || !f.changes.published[2].EvaluatedAt.Equal(h(5)) {
		t.Fatalf("fotos = %+v", f.changes.published)
	}
}

// Una hora que no cambia nada visible (mismo nivel, con datos) no publica.
func TestProcess_SameLevelHourPublishesNoChange(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	f.process(t, entities.SourceLive, notice(2, 0.18, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 3))

	if got := f.changes.changes(); !slices.Equal(got, []string{contract.ChangeOpened}) {
		t.Fatalf("cambios = %v", got)
	}
}

func TestProcess_FinishAndReactivationArePublished(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	end := notice(2, 0.12, entities.LevelNormal, entities.LevelNormal, entities.EpisodeCleared, 0)
	end.BelowRun = 2
	f.process(t, entities.SourceLive, end)
	f.process(t, entities.SourceLive,
		notice(8, 0.17, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 0),
		notice(9, 0.18, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeOpened, 2),
	)

	want := []string{contract.ChangeOpened, contract.ChangeFinished, contract.ChangeReactivated}
	if got := f.changes.changes(); !slices.Equal(got, want) {
		t.Fatalf("cambios = %v, se esperaban %v", got, want)
	}
	if c := f.changes.published[1]; c.State != "FINALIZADA" || c.EndedAt == nil || !c.EndedAt.Equal(h(2)) {
		t.Fatalf("foto del fin = %+v (ended_at %v)", c, c.EndedAt)
	}
	if c := f.changes.published[2]; c.State != "ACTIVA" || c.AlertID != f.changes.published[0].AlertID || c.EndedAt != nil {
		t.Fatalf("foto de la reactivación = %+v", c)
	}
}

func TestProcess_BackfillPublishesNoChanges(t *testing.T) {
	f := newFixture()
	f.now = h(200)
	f.process(t, entities.SourceBackfill, opening()...)
	if len(f.changes.published) != 0 {
		t.Fatalf("cambios = %v: el backfill nunca publica", f.changes.changes())
	}
}

func TestProcess_StoreFailurePublishesNoChanges(t *testing.T) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()[0])
	f.store.FailNextTransaction(errors.New("bd caída"))
	if _, err := f.svc.Process(context.Background(), opening()[1], entities.SourceLive); err == nil {
		t.Fatal("se esperaba error")
	}
	if len(f.changes.published) != 0 {
		t.Fatalf("cambios = %v: sin commit no se publica", f.changes.changes())
	}
}

// La foto se publica después del commit: si falla, la alerta queda guardada y
// el aviso igual se publica.
func TestProcess_ChangePublishFailureKeepsTheAlert(t *testing.T) {
	f := newFixture()
	f.changes.err = errors.New("bus caído")
	f.process(t, entities.SourceLive, opening()...)
	if len(f.store.Alerts()) != 1 || len(f.pub.published) != 1 {
		t.Fatalf("alertas = %d, avisos = %d", len(f.store.Alerts()), len(f.pub.published))
	}
}

// Sin WithChangePublisher el servicio funciona como antes.
func TestProcess_WithoutChangePublisherStillNotifies(t *testing.T) {
	f := newFixture()
	f.svc.WithChangePublisher(nil)
	f.process(t, entities.SourceLive, opening()...)
	if len(f.pub.published) != 1 || len(f.changes.published) != 0 {
		t.Fatalf("avisos = %d, cambios = %v", len(f.pub.published), f.changes.changes())
	}
}
