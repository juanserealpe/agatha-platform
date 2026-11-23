package services

import (
	"context"
	"errors"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

func newRebuildBackfill(history *fakeHistory, baselines *fakeBaselines, environment string) *BackfillService {
	return newGuardedBackfill(history, baselines, &fakeExclusions{}, environment)
}

func newGuardedBackfill(history *fakeHistory, baselines *fakeBaselines, exclusions *fakeExclusions, environment string) *BackfillService {
	return NewBackfillService(history, baselines, exclusions, newReplay(history, &fakeEvents{}), environment, testLogger())
}

func TestBackfill_RebuildReplacesTheExistingHistoryInOneStep(t *testing.T) {
	baselines := newFakeBaselines()
	baselines.latest[1] = &entities.BaselineVersion{Version: 37}
	baselines.replacedDeleted = 37

	got, err := newRebuildBackfill(replayHistory(), baselines, "development").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(650), Rebuild: true})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if !got.Created || got.Deleted != 37 || len(got.Versions) != 2 || got.Versions[0].ID != 200 {
		t.Fatalf("resultado = %+v", got)
	}
	if baselines.replacedPoint != 1 || len(baselines.replaced) != 2 {
		t.Fatalf("ReplaceHistory punto=%d versiones=%d", baselines.replacedPoint, len(baselines.replaced))
	}
	if baselines.history != nil {
		t.Fatal("con -rebuild no se usa SaveHistory")
	}
}

func TestBackfill_RebuildIsForbiddenInProduction(t *testing.T) {
	for _, env := range []string{"production", " Production ", "PRODUCTION"} {
		t.Run(env, func(t *testing.T) {
			history := replayHistory()
			baselines := newFakeBaselines()

			got, err := newRebuildBackfill(history, baselines, env).
				Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(650), Rebuild: true})

			if !errors.Is(err, domain.ErrRebuildForbidden) || got.Created {
				t.Fatalf("error=%v creado=%v, se esperaba ErrRebuildForbidden", err, got.Created)
			}
			if baselines.replaced != nil || len(history.calls) != 0 {
				t.Fatal("en production no se simula ni se escribe nada")
			}
		})
	}
}

func TestBackfill_BackfillWithoutRebuildStillWorksInProduction(t *testing.T) {
	baselines := newFakeBaselines()

	got, err := newRebuildBackfill(replayHistory(), baselines, "production").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(650)})
	if err != nil || !got.Created || baselines.history == nil {
		t.Fatalf("resultado=%+v err=%v", got, err)
	}
}

func TestBackfill_RebuildDryRunDoesNotWrite(t *testing.T) {
	baselines := newFakeBaselines()
	baselines.latest[1] = &entities.BaselineVersion{Version: 3}

	got, err := newRebuildBackfill(replayHistory(), baselines, "").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(500), DryRun: true, Rebuild: true})

	if err != nil || got.Created || got.SkippedReason != SkippedDryRun || baselines.replaced != nil {
		t.Fatalf("resultado=%+v err=%v", got, err)
	}
}

// Sin nada que poner en su lugar, el rebuild no borra el histórico existente.
func TestBackfill_RebuildWithoutSimulatedVersionsKeepsTheOldHistory(t *testing.T) {
	baselines := newFakeBaselines()
	baselines.latest[1] = &entities.BaselineVersion{Version: 3}

	got, err := newRebuildBackfill(&fakeHistory{}, baselines, "").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(500), Rebuild: true})

	if err != nil || got.SkippedReason != SkippedInsufficientHistory || baselines.replaced != nil {
		t.Fatalf("resultado=%+v err=%v reemplazado=%v", got, err, baselines.replaced)
	}
}

func TestBackfill_RebuildPersistFailure(t *testing.T) {
	baselines := newFakeBaselines()
	baselines.replaceErr = errDB

	got, err := newRebuildBackfill(replayHistory(), baselines, "").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(650), Rebuild: true})

	if !errors.Is(err, domain.ErrPersistingBaseline) || got.Created {
		t.Fatalf("error=%v creado=%v", err, got.Created)
	}
}

// Con una alerta vigente (fila ALERT sin ended_at) el rebuild aborta ANTES de
// simular: Alert Management tampoco dejaría reconstruir y la base quedaría a
// medias (Baseline nuevo, alertas viejas).
func TestBackfill_RebuildRefusesAPointWithAnActiveAlert(t *testing.T) {
	history := replayHistory()
	baselines := newFakeBaselines()
	exclusions := &fakeExclusions{open: map[int64]int{1: 1}}

	got, err := newGuardedBackfill(history, baselines, exclusions, "development").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(650), Rebuild: true})

	if !errors.Is(err, domain.ErrActiveAlertPresent) || got.Created {
		t.Fatalf("error=%v creado=%v, se esperaba ErrActiveAlertPresent", err, got.Created)
	}
	if len(exclusions.counted) != 1 || exclusions.counted[0] != 1 {
		t.Fatalf("puntos consultados = %v", exclusions.counted)
	}
	if baselines.replaced != nil || len(history.calls) != 0 {
		t.Fatal("con una alerta vigente no se simula ni se escribe nada")
	}
}

// La guarda es solo del rebuild real: el backfill normal y el dry-run no
// borran histórico ni cambian códigos de alertas.
func TestBackfill_ActiveAlertDoesNotBlockBackfillOrDryRun(t *testing.T) {
	tests := []struct {
		name string
		cmd  BackfillCommand
	}{
		{name: "backfill", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(650)}},
		{name: "rebuild dry-run", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(650), Rebuild: true, DryRun: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exclusions := &fakeExclusions{open: map[int64]int{1: 2}}
			_, err := newGuardedBackfill(replayHistory(), newFakeBaselines(), exclusions, "development").
				Backfill(context.Background(), tt.cmd)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if len(exclusions.counted) != 0 {
				t.Fatal("no debía consultar alertas vigentes")
			}
		})
	}
}

func TestBackfill_RebuildFailsIfActiveAlertsCannotBeCounted(t *testing.T) {
	baselines := newFakeBaselines()

	_, err := newGuardedBackfill(replayHistory(), baselines, &fakeExclusions{countErr: errDB}, "").
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(650), Rebuild: true})

	if !errors.Is(err, domain.ErrConsultingKnownEvents) || baselines.replaced != nil {
		t.Fatalf("error=%v reemplazado=%v", err, baselines.replaced)
	}
}
