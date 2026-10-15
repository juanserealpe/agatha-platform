package services_test

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

const pointID int64 = 11556

var (
	now        = time.Date(2026, 10, 15, 13, 10, 0, 0, time.UTC)
	readyUntil = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	// firstValidFrom deja dos horas evaluables: 11:00 y 12:00 (cuatro filas).
	firstValidFrom = time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC)
)

type fakeReader struct{}

func (fakeReader) GetSamples(context.Context, int64, time.Time, time.Time) ([]features.Sample, error) {
	return nil, nil
}
func (fakeReader) GetFirstTimestamp(context.Context, int64) (*time.Time, error) { return nil, nil }
func (fakeReader) GetLatestTimestamp(context.Context, int64) (*time.Time, error) {
	return &readyUntil, nil
}
func (fakeReader) ListActiveMeasuringPoints(context.Context) ([]int64, error) {
	return []int64{pointID}, nil
}

type fakeBaseline struct{}

func (fakeBaseline) GetVersions(context.Context, int64, time.Time, time.Time) ([]entities.BaselineSnapshot, error) {
	return nil, nil
}
func (fakeBaseline) GetFirstValidFrom(context.Context, int64) (*time.Time, error) {
	return &firstValidFrom, nil
}

// fakeEvaluations simula SaveAll: inserted < 0 ⇒ inserta todas las filas.
type fakeEvaluations struct {
	inserted int
	err      error
}

func (fakeEvaluations) GetLastEvaluatedAt(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (f fakeEvaluations) SaveAll(_ context.Context, evals []entities.FeatureEvaluation) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.inserted < 0 {
		return len(evals), nil
	}
	return f.inserted, nil
}

type fakePublisher struct {
	events []contract.IndicatorEvaluated
	err    error
}

func (p *fakePublisher) PublishIndicatorEvaluated(_ context.Context, e contract.IndicatorEvaluated) error {
	p.events = append(p.events, e)
	return p.err
}

func newService(evaluations fakeEvaluations, publisher ports.EventPublisher) *services.EvaluationService {
	return services.NewEvaluationService(
		fakeReader{},
		evaluations,
		fakeBaseline{},
		ports.ClockFunc(func() time.Time { return now }),
		services.DefaultConfig(),
		logger.New("ProcessingTest", "Error"),
		publisher,
	)
}

func evaluate(t *testing.T, svc *services.EvaluationService) (services.EvaluationResult, error) {
	t.Helper()
	return svc.EvaluatePending(context.Background(), services.EvaluatePendingCommand{
		PointID:    pointID,
		ReadyUntil: readyUntil,
		Trigger:    entities.TriggerReconciler,
		MaxSlots:   72,
	})
}

func TestEvaluatePending_PublishesOneEventPerPersistedWindow(t *testing.T) {
	publisher := &fakePublisher{}

	result, err := evaluate(t, newService(fakeEvaluations{inserted: -1}, publisher))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if result.Inserted != 4 {
		t.Fatalf("inserted = %d, se esperaban 4 filas (2 horas × 2 ventanas)", result.Inserted)
	}
	if len(publisher.events) != 4 {
		t.Fatalf("eventos = %d, se esperaban 4", len(publisher.events))
	}

	type key struct {
		window int
		at     time.Time
	}
	seen := map[key]bool{}
	for _, e := range publisher.events {
		seen[key{e.WindowHours, e.EvaluatedAt}] = true

		if e.MeasuringPointID != pointID {
			t.Errorf("measuring_point_id = %d", e.MeasuringPointID)
		}
		// Sin línea base vigente la ventana es NO_CALCULABLE y no tiene I.
		if e.Status != string(entities.StatusNoCalculable) || e.NoCalcReason != string(entities.ReasonNoSignalBaseline) || e.I != nil {
			t.Errorf("resultado = %s/%s/%v", e.Status, e.NoCalcReason, e.I)
		}
		if e.Trigger != string(entities.TriggerReconciler) {
			t.Errorf("trigger = %s", e.Trigger)
		}
		if !e.OccurredAt.Equal(now) || e.OccurredAt.Location() != time.UTC {
			t.Errorf("occurred_at = %v, se esperaba %v en UTC", e.OccurredAt, now)
		}
		if e.EvaluatedAt.Location() != time.UTC {
			t.Errorf("evaluated_at no está en UTC: %v", e.EvaluatedAt)
		}
		if e.SchemaVersion != contract.SchemaVersion {
			t.Errorf("schema_version = %d", e.SchemaVersion)
		}
	}

	for _, at := range []time.Time{firstValidFrom, readyUntil} {
		for _, w := range []int{entities.Window24h, entities.Window6h} {
			if !seen[key{w, at}] {
				t.Errorf("falta el evento de la ventana %d h de %v", w, at)
			}
		}
	}
}

func TestEvaluatePending_DoesNotPublishWhenNothingIsPersisted(t *testing.T) {
	tests := []struct {
		name        string
		evaluations fakeEvaluations
		wantErr     error
	}{
		{"falla el guardado (rollback)", fakeEvaluations{err: errors.New("db caída")}, domain.ErrPersistingEvaluations},
		{"otro proceso ya insertó todo", fakeEvaluations{inserted: 0}, nil},
		// Con un conflicto parcial no se sabe qué filas son las nuestras.
		{"conflicto parcial", fakeEvaluations{inserted: 2}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &fakePublisher{}

			_, err := evaluate(t, newService(tt.evaluations, publisher))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, se esperaba %v", err, tt.wantErr)
			}
			if len(publisher.events) != 0 {
				t.Fatalf("se publicaron %d eventos, se esperaba ninguno", len(publisher.events))
			}
		})
	}
}

func TestEvaluatePending_PublishFailureNeverFailsTheEvaluation(t *testing.T) {
	publisher := &fakePublisher{err: errors.New("bus caído")}

	result, err := evaluate(t, newService(fakeEvaluations{inserted: -1}, publisher))
	if err != nil {
		t.Fatalf("un fallo al publicar no debe fallar la evaluación: %v", err)
	}
	if result.Inserted != 4 || len(publisher.events) != 4 {
		t.Fatalf("inserted = %d, intentos de publicación = %d", result.Inserted, len(publisher.events))
	}
}

func TestEvaluatePending_WithoutPublisherDoesNotPublish(t *testing.T) {
	if _, err := evaluate(t, newService(fakeEvaluations{inserted: -1}, nil)); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestBackfill_DoesNotPublish(t *testing.T) {
	publisher := &fakePublisher{}

	result, err := newService(fakeEvaluations{inserted: -1}, publisher).
		Backfill(context.Background(), services.BackfillCommand{PointID: pointID})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if result.Inserted == 0 {
		t.Fatal("el backfill debía insertar filas")
	}
	if len(publisher.events) != 0 {
		t.Fatalf("el backfill publicó %d eventos; no debe publicar", len(publisher.events))
	}
}
