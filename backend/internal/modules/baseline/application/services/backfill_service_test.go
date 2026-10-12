package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const backfillPoint = 7

func newBackfillService(h *fakeHistory, b *fakeBaselines) *BackfillService {
	return NewBackfillService(h, b, newReplayService(h), logger.New("BaselineTest", "Error"))
}

// backfillHistory: 40 días de datos hasta asOf; el primer registro es asOf − 40 d.
func backfillHistory() *fakeHistory {
	return &fakeHistory{
		samples:   map[int64][]features.Sample{backfillPoint: noise(40)},
		firstSeen: ptrTime(asOf.AddDate(0, 0, -40)),
	}
}

func TestBackfillPersistsSimulatedHistory(t *testing.T) {
	h := backfillHistory()
	b := &fakeBaselines{}

	result, err := newBackfillService(h, b).Backfill(context.Background(), BackfillCommand{PointID: backfillPoint, To: asOf})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.SkippedReason != SkippedNone {
		t.Fatalf("created = %v, skipped = %q", result.Created, result.SkippedReason)
	}
	if !result.From.Equal(*h.firstSeen) || !result.To.Equal(asOf) {
		t.Fatalf("rango = [%v, %v), se esperaba desde el primer registro", result.From, result.To)
	}

	// Se guarda exactamente lo que simula el replay desde el primer registro.
	want, _, err := newReplayService(h).SimulateVersions(context.Background(), backfillPoint, *h.firstSeen, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) < 2 || len(b.history) != len(want) || len(result.Versions) != len(want) {
		t.Fatalf("simuladas = %d, guardadas = %d, devueltas = %d", len(want), len(b.history), len(result.Versions))
	}

	for i, v := range b.history {
		if !v.ValidFrom.Equal(want[i].ValidFrom) || !reflect.DeepEqual(v.Signal, want[i].Signal) || v.Trigger != want[i].Trigger {
			t.Fatalf("versión %d difiere de la simulada", i)
		}
		// La versión retroactiva se registra como si se hubiera creado en su momento.
		if !v.CreatedAt.Equal(v.ValidFrom) {
			t.Fatalf("versión %d: created_at = %v, valid_from = %v", i, v.CreatedAt, v.ValidFrom)
		}
		if i > 0 && !v.ValidFrom.After(b.history[i-1].ValidFrom) {
			t.Fatalf("versión %d fuera de orden cronológico", i)
		}
		if result.Versions[i].ID == 0 {
			t.Fatalf("versión %d devuelta sin ID", i)
		}
	}

	if b.history[0].Trigger != entities.TriggerBootstrap {
		t.Fatalf("primera versión = %s", b.history[0].Trigger)
	}
	if len(b.saved) != 0 {
		t.Fatal("el backfill no debe usar SaveNewVersion: el histórico se guarda en una sola TX")
	}
}

func TestBackfillFromExplicitDate(t *testing.T) {
	h := backfillHistory()
	b := &fakeBaselines{}
	from := asOf.AddDate(0, 0, -10)

	result, err := newBackfillService(h, b).Backfill(context.Background(),
		BackfillCommand{PointID: backfillPoint, From: from, To: asOf})
	if err != nil || len(b.history) == 0 {
		t.Fatalf("guardadas = %d, err = %v", len(b.history), err)
	}
	if !result.From.Equal(from) || b.history[0].ValidFrom.Before(from) {
		t.Fatalf("desde = %v, primera versión en %v", result.From, b.history[0].ValidFrom)
	}
}

func TestBackfillDryRunDoesNotPersist(t *testing.T) {
	h := backfillHistory()
	// Con versiones existentes el dry-run sigue permitido: solo muestra.
	b := &fakeBaselines{latest: map[int64]*entities.BaselineVersion{backfillPoint: {Version: 1}}}

	result, err := newBackfillService(h, b).Backfill(context.Background(),
		BackfillCommand{PointID: backfillPoint, To: asOf, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Created || result.SkippedReason != SkippedDryRun || len(result.Versions) < 2 {
		t.Fatalf("created = %v, skipped = %q, versiones = %d", result.Created, result.SkippedReason, len(result.Versions))
	}
	if b.history != nil {
		t.Fatal("el dry-run persistió versiones")
	}
}

func TestBackfillSkipsAndFails(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name        string
		history     *fakeHistory
		baselines   *fakeBaselines
		cmd         BackfillCommand
		wantErr     error
		wantSkipped SkippedReason
	}{
		{
			name:      "punto inválido",
			history:   backfillHistory(),
			baselines: &fakeBaselines{},
			cmd:       BackfillCommand{PointID: 0, To: asOf},
			wantErr:   domain.ErrInvalidMeasuringPointID,
		},
		{
			name:      "el punto ya tiene versiones",
			history:   backfillHistory(),
			baselines: &fakeBaselines{latest: map[int64]*entities.BaselineVersion{backfillPoint: {Version: 1}}},
			cmd:       BackfillCommand{PointID: backfillPoint, To: asOf},
			wantErr:   domain.ErrBaselineHistoryExists,
		},
		{
			name:      "error consultando la última versión",
			history:   backfillHistory(),
			baselines: &fakeBaselines{latestErr: map[int64]error{backfillPoint: boom}},
			cmd:       BackfillCommand{PointID: backfillPoint, To: asOf},
			wantErr:   domain.ErrConsultingBaseline,
		},
		{
			name:      "error consultando el primer registro",
			history:   &fakeHistory{firstErr: boom},
			baselines: &fakeBaselines{},
			cmd:       BackfillCommand{PointID: backfillPoint, To: asOf},
			wantErr:   domain.ErrConsultingHistory,
		},
		{
			name:        "dispositivo sin datos",
			history:     &fakeHistory{},
			baselines:   &fakeBaselines{},
			cmd:         BackfillCommand{PointID: backfillPoint, To: asOf},
			wantSkipped: SkippedInsufficientHistory,
		},
		{
			name:        "rango sin recálculos posibles",
			history:     backfillHistory(),
			baselines:   &fakeBaselines{},
			cmd:         BackfillCommand{PointID: backfillPoint, From: asOf.AddDate(0, 0, -60), To: asOf.AddDate(0, 0, -50)},
			wantSkipped: SkippedInsufficientHistory,
		},
		{
			name:      "otro proceso creó versiones durante el backfill",
			history:   backfillHistory(),
			baselines: &fakeBaselines{historyErr: domain.ErrBaselineHistoryExists},
			cmd:       BackfillCommand{PointID: backfillPoint, To: asOf},
			wantErr:   domain.ErrBaselineHistoryExists,
		},
		{
			name:      "error guardando el histórico",
			history:   backfillHistory(),
			baselines: &fakeBaselines{historyErr: boom},
			cmd:       BackfillCommand{PointID: backfillPoint, To: asOf},
			wantErr:   domain.ErrPersistingBaseline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := newBackfillService(tt.history, tt.baselines).Backfill(context.Background(), tt.cmd)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, se esperaba %v", err, tt.wantErr)
			}
			if result.Created {
				t.Fatal("no debía crear versiones")
			}
			if tt.wantErr == nil && result.SkippedReason != tt.wantSkipped {
				t.Fatalf("skipped = %q, se esperaba %q", result.SkippedReason, tt.wantSkipped)
			}
			if tt.wantErr == nil && tt.baselines.history != nil {
				t.Fatal("persistió versiones")
			}
		})
	}
}

// El primer recálculo cae en una hora en punto aunque el primer registro no lo esté.
func TestBackfillAlignsToHour(t *testing.T) {
	h := backfillHistory()
	h.firstSeen = ptrTime(h.firstSeen.Add(7 * time.Minute))
	b := &fakeBaselines{}

	if _, err := newBackfillService(h, b).Backfill(context.Background(), BackfillCommand{PointID: backfillPoint, To: asOf}); err != nil {
		t.Fatal(err)
	}
	for i, v := range b.history {
		if !v.ValidFrom.Equal(v.ValidFrom.Truncate(time.Hour)) {
			t.Fatalf("versión %d: valid_from = %v no es una hora en punto", i, v.ValidFrom)
		}
	}
}
