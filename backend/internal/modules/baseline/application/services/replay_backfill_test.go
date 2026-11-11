package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// replayHistory cubre 20 días de histórico antes de at(480) y 8 días después.
func replayHistory() *fakeHistory { return &fakeHistory{samples: normalSamples(at(0), at(480+192))} }

func newReplay(history *fakeHistory, events *fakeEvents) *ReplayService {
	return NewReplayService(history, events, testConfig(), testLogger())
}

func TestSimulateVersions_ChainsVersionsWeekByWeek(t *testing.T) {
	history := replayHistory()

	// Desde 479:30 (se redondea a 480) hasta 650: bootstrap en 480 y recálculo a las 168 h (648).
	got, err := newReplay(history, &fakeEvents{}).SimulateVersions(context.Background(), 1, at(479).Add(30*time.Minute), at(650))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(history.calls) != 1 || !history.calls[0].from.Equal(at(0)) || !history.calls[0].to.Equal(at(650)) {
		t.Fatalf("el histórico se carga una sola vez desde from − 20 días: %+v", history.calls)
	}
	if len(got) != 2 {
		t.Fatalf("se esperaban 2 versiones, hay %d", len(got))
	}

	v1, v2 := got[0], got[1]
	if v1.Version != 1 || v1.Trigger != entities.TriggerBootstrap || !v1.ValidFrom.Equal(at(480)) || !v1.CreatedAt.Equal(at(480)) {
		t.Fatalf("v1 mal armada: versión %d trigger %s desde %v creada %v", v1.Version, v1.Trigger, v1.ValidFrom, v1.CreatedAt)
	}
	if v1.ValidTo == nil || !v1.ValidTo.Equal(at(648)) {
		t.Fatalf("v1 debe cerrarse cuando empieza v2: %v", v1.ValidTo)
	}
	if v2.Version != 2 || v2.Trigger != entities.TriggerScheduled || !v2.ValidFrom.Equal(at(648)) || v2.ValidTo != nil {
		t.Fatalf("v2 mal armada: versión %d trigger %s desde %v hasta %v", v2.Version, v2.Trigger, v2.ValidFrom, v2.ValidTo)
	}
	if !v2.HistoryFrom.Equal(at(648-480)) || !v2.HistoryTo.Equal(at(648)) {
		t.Fatalf("v2 debe usar los 20 días previos: [%v, %v)", v2.HistoryFrom, v2.HistoryTo)
	}
}

func TestSimulateVersions_WithoutHistoryProducesNothing(t *testing.T) {
	got, err := newReplay(&fakeHistory{}, &fakeEvents{}).SimulateVersions(context.Background(), 1, at(480), at(500))

	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("versiones=%v err=%v, se esperaba lista vacía", got, err)
	}
}

func TestSimulateVersions_Errors(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		pointID int64
		history *fakeHistory
		events  *fakeEvents
		want    error
	}{
		{name: "punto inválido", ctx: context.Background(), pointID: -1, history: replayHistory(), events: &fakeEvents{}, want: domain.ErrInvalidMeasuringPointID},
		{name: "falla el histórico", ctx: context.Background(), pointID: 1, history: &fakeHistory{err: errDB}, events: &fakeEvents{}, want: domain.ErrConsultingHistory},
		{name: "fallan los eventos", ctx: context.Background(), pointID: 1, history: replayHistory(), events: &fakeEvents{err: errDB}, want: domain.ErrConsultingKnownEvents},
		{name: "contexto cancelado", ctx: canceled, pointID: 1, history: replayHistory(), events: &fakeEvents{}, want: context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newReplay(tt.history, tt.events).SimulateVersions(tt.ctx, tt.pointID, at(480), at(500))
			if got != nil || !errors.Is(err, tt.want) {
				t.Fatalf("versiones=%v error=%v, se esperaba %v", got, err, tt.want)
			}
		})
	}
}

func newBackfill(history *fakeHistory, baselines *fakeBaselines) *BackfillService {
	return NewBackfillService(history, baselines, newReplay(history, &fakeEvents{}), testLogger())
}

func TestBackfill_SavesTheSimulatedHistoryFromTheFirstRecord(t *testing.T) {
	history := replayHistory()
	first := at(480)
	history.firstSeen = &first
	baselines := newFakeBaselines()

	got, err := newBackfill(history, baselines).Backfill(context.Background(), BackfillCommand{PointID: 1, To: at(650)})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if !got.Created || got.SkippedReason != "" || !got.From.Equal(first) || !got.To.Equal(at(650)) {
		t.Fatalf("resultado = %+v", got)
	}
	if len(baselines.history) != 2 || len(got.Versions) != 2 || got.Versions[0].ID != 100 {
		t.Fatalf("se debe devolver lo que guardó el repositorio: %+v", got.Versions)
	}
}

func TestBackfill_DryRunSimulatesWithoutCheckingOrSaving(t *testing.T) {
	baselines := newFakeBaselines()
	baselines.latest[1] = &entities.BaselineVersion{Version: 3} // ya hay versiones, pero el dry run no mira

	got, err := newBackfill(replayHistory(), baselines).
		Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(500), DryRun: true})

	if err != nil || got.Created || got.SkippedReason != SkippedDryRun || len(got.Versions) != 1 || baselines.history != nil {
		t.Fatalf("resultado=%+v err=%v guardado=%v", got, err, baselines.history)
	}
}

func TestBackfill_Skips(t *testing.T) {
	t.Run("sin primer registro", func(t *testing.T) {
		got, err := newBackfill(&fakeHistory{}, newFakeBaselines()).Backfill(context.Background(), BackfillCommand{PointID: 1, To: at(500)})
		if err != nil || got.SkippedReason != SkippedInsufficientHistory || got.Created {
			t.Fatalf("resultado=%+v err=%v", got, err)
		}
	})

	t.Run("la simulación no produce versiones", func(t *testing.T) {
		baselines := newFakeBaselines()
		got, err := newBackfill(&fakeHistory{}, baselines).
			Backfill(context.Background(), BackfillCommand{PointID: 1, From: at(480), To: at(500)})
		if err != nil || got.SkippedReason != SkippedInsufficientHistory || baselines.history != nil {
			t.Fatalf("resultado=%+v err=%v", got, err)
		}
	})
}

func TestBackfill_Errors(t *testing.T) {
	tests := []struct {
		name      string
		cmd       BackfillCommand
		history   func() *fakeHistory
		baselines func() *fakeBaselines
		want      error
	}{
		{name: "punto inválido", cmd: BackfillCommand{PointID: 0}, want: domain.ErrInvalidMeasuringPointID},
		{
			name: "el punto ya tiene versiones", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(500)},
			baselines: func() *fakeBaselines {
				b := newFakeBaselines()
				b.latest[1] = &entities.BaselineVersion{Version: 1}
				return b
			},
			want: domain.ErrBaselineHistoryExists,
		},
		{
			name: "falla la consulta de la última versión", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(500)},
			baselines: func() *fakeBaselines { b := newFakeBaselines(); b.latestErr[1] = errDB; return b },
			want:      domain.ErrConsultingBaseline,
		},
		{
			name: "falla el primer registro", cmd: BackfillCommand{PointID: 1, To: at(500)},
			history: func() *fakeHistory { h := replayHistory(); h.firstErr = errDB; return h },
			want:    domain.ErrConsultingHistory,
		},
		{
			name: "falla la simulación", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(500)},
			history: func() *fakeHistory { return &fakeHistory{err: errDB} },
			want:    domain.ErrConsultingHistory,
		},
		{
			name: "otro proceso creó versiones al guardar", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(500)},
			baselines: func() *fakeBaselines {
				b := newFakeBaselines()
				b.historyEr = fmt.Errorf("tx: %w", domain.ErrBaselineHistoryExists)
				return b
			},
			want: domain.ErrBaselineHistoryExists,
		},
		{
			name: "falla al guardar", cmd: BackfillCommand{PointID: 1, From: at(480), To: at(500)},
			baselines: func() *fakeBaselines { b := newFakeBaselines(); b.historyEr = errDB; return b },
			want:      domain.ErrPersistingBaseline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history, baselines := replayHistory(), newFakeBaselines()
			if tt.history != nil {
				history = tt.history()
			}
			if tt.baselines != nil {
				baselines = tt.baselines()
			}

			got, err := newBackfill(history, baselines).Backfill(context.Background(), tt.cmd)
			if !errors.Is(err, tt.want) || got.Created {
				t.Fatalf("error=%v creado=%v, se esperaba %v", err, got.Created, tt.want)
			}
		})
	}
}
