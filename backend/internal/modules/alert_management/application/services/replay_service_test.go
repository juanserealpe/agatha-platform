package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

type fakeLevels struct {
	rows map[int64][]entities.LevelNotice
}

func (f fakeLevels) ListPoints(context.Context) ([]int64, error) {
	var out []int64
	for p := range f.rows {
		out = append(out, p)
	}
	return out, nil
}

func (f fakeLevels) ListAfter(_ context.Context, pointID int64, after, to time.Time, limit int) ([]entities.LevelNotice, error) {
	var out []entities.LevelNotice
	for _, n := range f.rows[pointID] {
		if (after.IsZero() || n.EvaluatedAt.After(after)) && (to.IsZero() || n.EvaluatedAt.Before(to)) {
			out = append(out, n)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// La fuga del escenario: abre, sube y cierra; con un hueco de 2 h (sin fila).
func leakRows() []entities.LevelNotice {
	rows := opening()
	rows = append(rows,
		notice(2, 0.27, entities.LevelNaranja, entities.LevelNaranja, entities.EpisodeNone, 3),
		notice(5, 0.20, entities.LevelAmarillo, entities.LevelAmarillo, entities.EpisodeNone, 4),
	)
	end := notice(6, 0.12, entities.LevelNormal, entities.LevelNormal, entities.EpisodeCleared, 0)
	end.BelowRun = 2
	return append(rows, end)
}

func newReplay(f *fixture, levels fakeLevels, env string) *services.ReplayService {
	return services.NewReplayService(f.svc, levels, f.store, services.ReplayConfig{Environment: env, BatchSize: 2}, testLogger())
}

func TestBackfill_RebuildsHistoryWithoutNotifying(t *testing.T) {
	f := newFixture()
	f.now = h(500)
	levels := fakeLevels{rows: map[int64][]entities.LevelNotice{pointA: leakRows()}}

	result, err := newReplay(f, levels, "development").Backfill(context.Background(), services.BackfillCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Opened != 1 || result.Finished != 1 || result.Hours != 7 {
		t.Fatalf("resumen = %+v (5 filas + 2 horas sin cálculo)", result)
	}
	a := f.store.Alerts()[0]
	if a.State != entities.AlertFinished || a.MaxLevel != entities.LevelNaranja || a.HoursNoData != 2 || a.Source != entities.SourceBackfill {
		t.Fatalf("alerta = %+v", a)
	}
	if len(f.pub.published) != 0 {
		t.Fatal("el backfill no notifica")
	}

	// Idempotente: repetirlo no hace nada.
	again, err := newReplay(f, levels, "development").Backfill(context.Background(), services.BackfillCommand{})
	if err != nil || again.Hours != 0 {
		t.Fatalf("segunda corrida = %+v, %v", again, err)
	}
}

func TestCatchUp_SkipsPointsWithoutHistoryAndNotifiesTheRest(t *testing.T) {
	f := newFixture()
	levels := fakeLevels{rows: map[int64][]entities.LevelNotice{pointA: leakRows()}}
	replay := newReplay(f, levels, "development")

	summary, err := replay.CatchUp(context.Background())
	if err != nil || len(summary) != 1 || !summary[0].WithoutHistory {
		t.Fatalf("resumen = %+v, %v: un punto sin estado espera al backfill", summary, err)
	}

	// Con el inicio en el historial, el catch-up procesa lo que falta y notifica.
	f.now = h(0).Add(35 * time.Minute)
	f.process(t, entities.SourceLive, leakRows()[0])
	f.now = h(1).Add(50 * time.Minute)
	summary, err = replay.CatchUp(context.Background())
	if err != nil || summary[0].Hours != 6 {
		t.Fatalf("resumen = %+v, %v", summary, err)
	}
	if a := f.store.Alerts()[0]; a.Source != entities.SourceCatchUp {
		t.Fatalf("source = %s", a.Source)
	}
	if len(f.pub.published) == 0 {
		t.Fatal("la apertura recuperada a tiempo se publica")
	}
}

func TestBackfill_RebuildForbiddenInProduction(t *testing.T) {
	f := newFixture()
	_, err := newReplay(f, fakeLevels{}, "production").Backfill(context.Background(), services.BackfillCommand{Rebuild: true})
	if err != domain.ErrRebuildForbidden {
		t.Fatalf("err = %v", err)
	}
}

func TestBackfill_RebuildStartsOver(t *testing.T) {
	f := newFixture()
	f.now = h(500)
	levels := fakeLevels{rows: map[int64][]entities.LevelNotice{pointA: leakRows()}}
	replay := newReplay(f, levels, "development")
	if _, err := replay.Backfill(context.Background(), services.BackfillCommand{}); err != nil {
		t.Fatal(err)
	}
	result, err := replay.Backfill(context.Background(), services.BackfillCommand{PointID: pointA, Rebuild: true})
	if err != nil || result.Deleted != 1 || result.Opened != 1 || len(f.store.Alerts()) != 1 {
		t.Fatalf("rebuild = %+v, %v, alertas %d", result, err, len(f.store.Alerts()))
	}
}
