package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
)

type simulationFixture struct {
	thresholds *memory.ThresholdRepository
	source     *fakeSource
	sandboxes  []*memory.RuleEvaluationRepository
	svc        *services.SimulationService
}

func newSimulationFixture() *simulationFixture {
	f := &simulationFixture{
		thresholds: memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, referenceParams())),
		source:     newSource(),
	}
	sandbox := func(seed entities.ThresholdVersion) (repositories.ThresholdRepository, repositories.RuleEvaluationRepository) {
		evaluations := memory.NewRuleEvaluationRepository()
		f.sandboxes = append(f.sandboxes, evaluations)
		return memory.NewThresholdRepository(seed), evaluations
	}
	f.svc = services.NewSimulationService(f.thresholds, f.source, sandbox, fixedClock(), testLogger())
	return f
}

func simulateAmarillo016(f *simulationFixture) (services.SimulationResult, error) {
	return f.svc.Simulate(context.Background(), services.SimulateCommand{
		PointID: pointA,
		Patch:   entities.ThresholdPatch{Amarillo: entities.Some(0.16)},
		From:    hour(0),
		To:      hour(48),
	})
}

// Caso 11 (sintético): bajar amarillo a 0,16 agrega un episodio que con 0,18
// no existía; no se escribe ninguna versión ni fila real.
func TestSimulate_AmarilloChangeAddsEpisode(t *testing.T) {
	f := newSimulationFixture()
	// Episodio que ambas versiones ven: 0,20 × 3, después salida.
	// Episodio que solo ve 0,16: 0,17 × 3, después salida.
	f.source.add(pointA, 0.20, 0.20, 0.20, 0.10, 0.10, 0.10, 0.17, 0.17, 0.17, 0.10, 0.10)

	got, err := simulateAmarillo016(f)
	if err != nil {
		t.Fatal(err)
	}

	if got.HoursEvaluated != 11 || len(got.Episodes) != 2 || len(got.CurrentEpisodes) != 1 {
		t.Fatalf("horas = %d, episodios = %d (vigentes %d); se esperaban 11, 2 y 1",
			got.HoursEvaluated, len(got.Episodes), len(got.CurrentEpisodes))
	}
	if got.NewEpisodes != 1 || got.RemovedEpisodes != 0 {
		t.Fatalf("new = %d, removed = %d; se esperaban 1 y 0", got.NewEpisodes, got.RemovedEpisodes)
	}
	if got.Proposed.Amarillo != 0.16 || got.CurrentVersion.Amarillo != 0.18 {
		t.Fatalf("propuesto %v / vigente %v", got.Proposed.Amarillo, got.CurrentVersion.Amarillo)
	}

	e := got.Episodes[1]
	if !e.OpenedAt.Equal(hour(7)) || e.ClearedAt == nil || !e.ClearedAt.Equal(hour(10)) || e.MaxLevel != entities.LevelAmarillo || e.Hours != 5 {
		t.Fatalf("episodio nuevo = %+v", e)
	}
	if got.ByMaxLevel[entities.LevelAmarillo] != 2 {
		t.Fatalf("by_max_level = %v", got.ByMaxLevel)
	}

	if versions := f.thresholds.All(); len(versions) != 1 || versions[0].ValidTo != nil {
		t.Fatal("la simulación no crea ni cierra versiones reales")
	}
	if len(f.sandboxes) != 2 {
		t.Fatalf("sandboxes = %d; se esperaban 2 (propuesto y vigente), uno por corrida", len(f.sandboxes))
	}
}

func TestSimulate_EpisodeOpenAtEndAndLevels(t *testing.T) {
	f := newSimulationFixture()
	f.source.add(pointA, 0.20, 0.20, 0.30, 0.40)

	got, err := simulateAmarillo016(f)
	if err != nil {
		t.Fatal(err)
	}
	e := got.Episodes[0]
	if e.ClearedAt != nil || e.MaxLevel != entities.LevelRojo || e.FirstNaranjaAt == nil || !e.FirstNaranjaAt.Equal(hour(2)) ||
		e.FirstRojoAt == nil || !e.FirstRojoAt.Equal(hour(3)) {
		t.Fatalf("episodio = %+v", e)
	}
}

func TestSimulate_Validation(t *testing.T) {
	f := newSimulationFixture()
	ctx := context.Background()

	tests := []struct {
		name string
		cmd  services.SimulateCommand
		want error
	}{
		{"rango invertido", services.SimulateCommand{PointID: pointA, From: hour(5), To: hour(1)}, domain.ErrInvalidTimeRange},
		{"sin rango", services.SimulateCommand{PointID: pointA}, domain.ErrInvalidTimeRange},
		{"rango largo", services.SimulateCommand{PointID: pointA, From: hour(0), To: hour(0).Add(367 * 24 * time.Hour)}, domain.ErrSimulationRangeTooLong},
		{"parche inválido", services.SimulateCommand{
			PointID: pointA, From: hour(0), To: hour(1), Patch: entities.ThresholdPatch{Amarillo: entities.Some(0.26)},
		}, domain.ErrThresholdOrder},
		{"punto sin umbrales", services.SimulateCommand{PointID: pointB, From: hour(0), To: hour(1)}, domain.ErrThresholdsNotFound},
		{"punto inválido", services.SimulateCommand{From: hour(0), To: hour(1)}, domain.ErrInvalidMeasuringPointID},
	}
	for _, tt := range tests {
		if _, err := f.svc.Simulate(ctx, tt.cmd); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, se esperaba %v", tt.name, err, tt.want)
		}
	}
}

func TestEpisodesFromRows_IgnoresRowsOutsideEpisodes(t *testing.T) {
	rows := []entities.RuleEvaluation{
		{EvaluatedAt: hour(0), Level: entities.LevelNormal, EpisodeEvent: entities.EpisodeEventNone},
		// La fila CLEARED de un episodio abierto antes del rango no abre nada.
		{EvaluatedAt: hour(1), Level: entities.LevelNormal, EpisodeEvent: entities.EpisodeEventCleared},
	}
	if eps := services.EpisodesFromRows(rows); len(eps) != 0 {
		t.Fatalf("episodios = %+v", eps)
	}
}
