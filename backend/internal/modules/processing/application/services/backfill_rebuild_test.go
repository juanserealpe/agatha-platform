package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
)

func TestBackfill_RebuildReplacesTheWholeHistoryInOneStep(t *testing.T) {
	call := &replaceCall{deleted: 7}
	publisher := &fakePublisher{}

	got, err := newServiceIn("development", fakeEvaluations{replace: call}, publisher).
		Backfill(context.Background(), services.BackfillCommand{PointID: pointID, Rebuild: true})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	// Sin -from se borra todo lo anterior: from = nil, to = última hora planificada.
	if !call.called || call.from != nil || !call.to.Equal(readyUntil) || call.rows != 4 {
		t.Fatalf("ReplaceRange = %+v", call)
	}
	if got.Deleted != 7 || got.Inserted != 4 {
		t.Fatalf("resultado: deleted=%d inserted=%d", got.Deleted, got.Inserted)
	}
	if len(publisher.events) != 0 {
		t.Fatal("el rebuild no publica eventos")
	}
}

func TestBackfill_RebuildWithFromOnlyReplacesFromThatHour(t *testing.T) {
	call := &replaceCall{}

	_, err := newServiceIn("", fakeEvaluations{replace: call}, nil).Backfill(context.Background(),
		services.BackfillCommand{PointID: pointID, From: readyUntil.Add(-30 * time.Minute), Rebuild: true})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if call.from == nil || !call.from.Equal(readyUntil) || call.rows != 2 {
		t.Fatalf("ReplaceRange = %+v (from %v)", call, call.from)
	}
}

func TestBackfill_RebuildIsForbiddenInProduction(t *testing.T) {
	call := &replaceCall{}

	got, err := newServiceIn(" Production ", fakeEvaluations{replace: call}, nil).
		Backfill(context.Background(), services.BackfillCommand{PointID: pointID, Rebuild: true})

	if !errors.Is(err, domain.ErrRebuildForbidden) || call.called || got.Inserted != 0 {
		t.Fatalf("err=%v llamado=%v", err, call.called)
	}
}

func TestBackfill_RebuildDryRunDoesNotWrite(t *testing.T) {
	call := &replaceCall{}

	got, err := newServiceIn("", fakeEvaluations{replace: call}, nil).
		Backfill(context.Background(), services.BackfillCommand{PointID: pointID, Rebuild: true, DryRun: true})

	if err != nil || call.called || len(got.Evaluations) != 4 {
		t.Fatalf("err=%v llamado=%v filas=%d", err, call.called, len(got.Evaluations))
	}
}

func TestBackfill_RebuildPersistFailure(t *testing.T) {
	call := &replaceCall{err: errors.New("db caída")}

	_, err := newServiceIn("", fakeEvaluations{replace: call}, nil).
		Backfill(context.Background(), services.BackfillCommand{PointID: pointID, Rebuild: true})

	if !errors.Is(err, domain.ErrPersistingEvaluations) {
		t.Fatalf("err=%v", err)
	}
}
