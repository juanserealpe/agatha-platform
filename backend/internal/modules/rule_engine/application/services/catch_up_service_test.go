package services_test

import (
	"context"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

func newCatchUp(f *fixture, source *fakeSource, batch int) *services.CatchUpService {
	return services.NewCatchUpService(f.svc, source, f.evaluations, batch, testLogger())
}

// backfillFirst interpreta (sin publicar) las primeras n horas de la fuente,
// como lo deja rule_engine -backfill.
func backfillFirst(t *testing.T, f *fixture, source *fakeSource, n int) {
	t.Helper()
	_, err := newBackfill(f, source, "development", 500).Backfill(context.Background(),
		services.BackfillCommand{PointID: pointA, To: hour(n)})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCatchUp_InterpretsAndPublishesTheHoursAfterTheLastRow(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)
	backfillFirst(t, f, source, 4)

	summary, err := newCatchUp(f, source, 3).CatchUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(summary) != 1 || summary[0].Hours != 4 || summary[0].Saved != 4 || summary[0].Published != 4 || summary[0].Err != nil {
		t.Fatalf("resumen = %+v", summary)
	}
	if len(f.publisher.events) != 4 {
		t.Fatalf("eventos = %d, se esperaban 4 (solo las horas sin fila)", len(f.publisher.events))
	}
	for n, e := range f.publisher.events {
		if !e.EvaluatedAt.Equal(hour(4 + n)) {
			t.Fatalf("evento %d en %v, se esperaba %v (en orden)", n, e.EvaluatedAt, hour(4+n))
		}
	}
	// Las rachas siguen desde las filas del backfill: 0,15 y 0,14 (horas 5 y 6)
	// cumplen exit_hours y cierran el episodio abierto en el backfill.
	if e := f.publisher.events[0]; !e.InEpisode || e.EpisodeLevel != string(entities.LevelNaranja) {
		t.Fatalf("hora 4 = %+v, se esperaba el episodio NARANJA del backfill en HOLD", e)
	}
	if e := f.publisher.events[2]; e.EpisodeEvent != string(entities.EpisodeEventCleared) {
		t.Fatalf("hora 6 = %s, se esperaba CLEARED", e.EpisodeEvent)
	}
}

func TestCatchUp_IsIdempotent(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)
	backfillFirst(t, f, source, 4)
	catchUp := newCatchUp(f, source, 500)

	if _, err := catchUp.CatchUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	summary, err := catchUp.CatchUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if summary[0].Hours != 0 || len(f.publisher.events) != 4 {
		t.Fatalf("segunda corrida = %+v, eventos = %d; se esperaba nada nuevo", summary, len(f.publisher.events))
	}
}

func TestCatchUp_SkipsPointsWithoutHistory(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)

	summary, err := newCatchUp(f, source, 500).CatchUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(summary) != 1 || !summary[0].WithoutHistory || summary[0].Hours != 0 {
		t.Fatalf("resumen = %+v: un punto sin filas espera a rule_engine -backfill", summary)
	}
	if len(f.publisher.events) != 0 {
		t.Fatalf("eventos = %d: nunca se publica el histórico completo", len(f.publisher.events))
	}
}
