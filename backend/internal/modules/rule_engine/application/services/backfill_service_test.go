package services_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// fakeSource simula feature_evaluations (solo horas CALCULATED de 24 h).
type fakeSource struct {
	rows  map[int64][]ports.CalculatedIndicator
	calls int
	// dPl es el d_pl por punto y hora que devuelve GetDPl (evaluación en vivo).
	dPl    map[int64]map[time.Time]float64
	dPlErr error
}

func newSource() *fakeSource {
	return &fakeSource{rows: map[int64][]ports.CalculatedIndicator{}, dPl: map[int64]map[time.Time]float64{}}
}

func (s *fakeSource) setDPl(pointID int64, at time.Time, v float64) {
	if s.dPl[pointID] == nil {
		s.dPl[pointID] = map[time.Time]float64{}
	}
	s.dPl[pointID][at] = v
}

func (s *fakeSource) GetDPl(_ context.Context, pointID int64, at time.Time) (*float64, error) {
	if s.dPlErr != nil {
		return nil, s.dPlErr
	}
	if v, ok := s.dPl[pointID][at]; ok {
		return &v, nil
	}
	return nil, nil
}

func (s *fakeSource) add(pointID int64, values ...float64) {
	for n, i := range values {
		s.rows[pointID] = append(s.rows[pointID], ports.CalculatedIndicator{MeasuringPointID: pointID, EvaluatedAt: hour(n), I: i})
	}
}

func inRange(at, from, to time.Time) bool {
	return (from.IsZero() || !at.Before(from)) && (to.IsZero() || at.Before(to))
}

func (s *fakeSource) ListPoints(_ context.Context, from, to time.Time) ([]int64, error) {
	var points []int64
	for p, rows := range s.rows {
		for _, r := range rows {
			if inRange(r.EvaluatedAt, from, to) {
				points = append(points, p)
				break
			}
		}
	}
	slices.Sort(points)
	return points, nil
}

func (s *fakeSource) ListCalculated(_ context.Context, pointID int64, from, to, after time.Time, limit int) ([]ports.CalculatedIndicator, error) {
	s.calls++
	var out []ports.CalculatedIndicator
	for _, r := range s.rows[pointID] {
		if inRange(r.EvaluatedAt, from, to) && (after.IsZero() || r.EvaluatedAt.After(after)) {
			out = append(out, r)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func newBackfill(f *fixture, source *fakeSource, env string, batch int) *services.BackfillService {
	return services.NewBackfillService(f.svc, source, f.evaluations,
		services.BackfillConfig{Environment: env, BatchSize: batch}, testLogger())
}

// La secuencia cruza el umbral, sube a NARANJA y cierra: 1 episodio abierto y cerrado.
var crossing = []float64{0.12, 0.19, 0.20, 0.30, 0.17, 0.15, 0.14, 0.10}

func TestBackfill_InterpretsInOrderInBatchesWithoutPublishing(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)

	result, err := newBackfill(f, source, "development", 3).Backfill(context.Background(), services.BackfillCommand{PointID: pointA})
	if err != nil {
		t.Fatal(err)
	}

	if source.calls < 3 {
		t.Fatalf("lecturas = %d: con lotes de 3 y 8 horas debían ser al menos 3", source.calls)
	}
	if result.Hours != 8 || result.Saved != 8 || result.EpisodesOpened != 1 || result.EpisodesClosed != 1 {
		t.Fatalf("resumen = %+v", result)
	}
	if result.HoursByLevel[entities.LevelNormal] != 4 || result.HoursByLevel[entities.LevelAmarillo] != 1 || result.HoursByLevel[entities.LevelNaranja] != 3 {
		t.Fatalf("horas por nivel = %v", result.HoursByLevel)
	}

	// Mismas filas que la evaluación en vivo (misma lógica, mismo orden).
	live := newFixture()
	live.feed(t, crossing...)
	got, want := f.evaluations.Rows(pointA), live.evaluations.Rows(pointA)
	for n := range want {
		if got[n].Level != want[n].Level || got[n].EpisodeEvent != want[n].EpisodeEvent || got[n].ComputedLevel != want[n].ComputedLevel || got[n].HoursAbove != want[n].HoursAbove {
			t.Fatalf("hora %d: backfill %+v != en vivo %+v", n, got[n], want[n])
		}
	}

	if len(f.publisher.events) != 0 {
		t.Fatal("el backfill NUNCA publica")
	}
}

func TestBackfill_IsIdempotent(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)
	backfill := newBackfill(f, source, "development", 100)

	if _, err := backfill.Backfill(context.Background(), services.BackfillCommand{PointID: pointA}); err != nil {
		t.Fatal(err)
	}
	before := f.evaluations.Rows(pointA)

	again, err := backfill.Backfill(context.Background(), services.BackfillCommand{PointID: pointA})
	if err != nil {
		t.Fatal(err)
	}
	if again.Saved != 0 || again.Duplicates != 8 {
		t.Fatalf("segunda corrida = %+v; se esperaba 0 guardadas y 8 duplicadas", again)
	}
	if after := f.evaluations.Rows(pointA); !reflect.DeepEqual(before, after) {
		t.Fatal("repetir el backfill no debe cambiar las filas")
	}
}

func TestBackfill_UsesEachVersionInTheRange(t *testing.T) {
	change := hour(4)
	strict := referenceParams()
	strict.Amarillo, strict.Naranja, strict.Rojo = 0.40, 0.50, 0.60
	f := newFixture(
		seedVersion(pointA, 1, day1, &change, referenceParams()),
		seedVersion(pointA, 2, change, nil, strict),
	)
	source := newSource()
	source.add(pointA, 0.30, 0.30, 0.30, 0.30, 0.30, 0.30, 0.30, 0.30)

	if _, err := newBackfill(f, source, "development", 100).Backfill(context.Background(), services.BackfillCommand{PointID: pointA}); err != nil {
		t.Fatal(err)
	}

	v1, v2 := f.thresholds.All()[0].ID, f.thresholds.All()[1].ID
	for n, row := range f.evaluations.Rows(pointA) {
		want := v1
		if n >= 4 {
			want = v2
		}
		if row.ThresholdVersionID != want {
			t.Fatalf("hora %d: versión %d, se esperaba %d", n, row.ThresholdVersionID, want)
		}
	}
}

func TestBackfill_AllPointsAndRange(t *testing.T) {
	f := newFixture(
		seedVersion(pointA, 1, day1, nil, referenceParams()),
		seedVersion(pointB, 1, day1, nil, referenceParams()),
	)
	source := newSource()
	source.add(pointA, crossing...)
	source.add(pointB, 0.10, 0.10, 0.10)

	result, err := newBackfill(f, source, "development", 100).Backfill(context.Background(), services.BackfillCommand{
		From: hour(1), To: hour(3), // [1, 3): dos horas por punto
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Points) != 2 || result.Hours != 4 {
		t.Fatalf("resumen = %+v", result)
	}
	if rows := f.evaluations.Rows(pointA); len(rows) != 2 || !rows[0].EvaluatedAt.Equal(hour(1)) {
		t.Fatalf("filas A = %+v", rows)
	}
}

func TestBackfill_RebuildIsBlockedInProduction(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)
	if _, err := newBackfill(f, source, "development", 100).Backfill(context.Background(), services.BackfillCommand{PointID: pointA}); err != nil {
		t.Fatal(err)
	}

	for _, env := range []string{"production", "PRODUCTION", " production "} {
		_, err := newBackfill(f, source, env, 100).Backfill(context.Background(), services.BackfillCommand{PointID: pointA, Rebuild: true})
		if !errors.Is(err, domain.ErrRebuildForbidden) {
			t.Fatalf("AGATHA_ENV=%q: err = %v, se esperaba ErrRebuildForbidden", env, err)
		}
	}
	if rows := f.evaluations.Rows(pointA); len(rows) != 8 {
		t.Fatalf("filas = %d: en production no se borra nada", len(rows))
	}
}

func TestBackfill_RebuildDeletesTheRangeAndRecomputes(t *testing.T) {
	f := newFixture()
	source := newSource()
	source.add(pointA, crossing...)
	ctx := context.Background()

	// Una fila previa "vieja" (p. ej. interpretada con un orden distinto).
	stale := calculated(pointA, hour(2), 0.90)
	if _, err := f.svc.EvaluateIndicatorWithoutPublishing(ctx, stale); err != nil {
		t.Fatal(err)
	}

	result, err := newBackfill(f, source, "", 100).Backfill(ctx, services.BackfillCommand{PointID: pointA, Rebuild: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 1 || result.Saved != 8 {
		t.Fatalf("resumen = %+v", result)
	}
	if row := f.evaluations.Rows(pointA)[2]; row.IndicatorValue != 0.20 || row.Level != entities.LevelAmarillo {
		t.Fatalf("la hora 2 debía recalcularse desde feature_evaluations: %+v", row)
	}
}

func TestBackfill_InvalidRange(t *testing.T) {
	f := newFixture()
	_, err := newBackfill(f, newSource(), "", 100).Backfill(context.Background(), services.BackfillCommand{From: hour(5), To: hour(1)})
	if !errors.Is(err, domain.ErrInvalidTimeRange) {
		t.Fatalf("err = %v", err)
	}
}
