package services

import (
	"context"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// fakeUnitOfWork ejecuta fn con repositorios en memoria y registra el orden de
// las escrituras. No simula el rollback: eso lo cubre el test de integración.
type fakeUnitOfWork struct {
	measurements *fakeMeasurementRepository
	states       *fakeIngestionStateRepository
	// fnErr es lo que devolvió fn: distinto de nil = el UoW real revertiría.
	fnErr error
}

func newFakeUnitOfWork(batchSeq int64) *fakeUnitOfWork {
	calls := &[]string{}
	return &fakeUnitOfWork{
		measurements: &fakeMeasurementRepository{calls: calls},
		states:       &fakeIngestionStateRepository{calls: calls, batchSeq: batchSeq, created: true},
	}
}

func (u *fakeUnitOfWork) Execute(ctx context.Context, fn func(
	ctx context.Context,
	measurementRepository repositories.MeasurementRepository,
	ingestionStateRepository repositories.IngestionStateRepository,
) error) error {
	u.fnErr = fn(ctx, u.measurements, u.states)
	return u.fnErr
}

func (u *fakeUnitOfWork) calls() []string { return *u.measurements.calls }

type fakeMeasurementRepository struct {
	calls *[]string
	saved []entities.RawMeasurement
	// allDuplicates simula que todas las filas chocan con el índice único.
	allDuplicates bool
	err           error
}

func (r *fakeMeasurementRepository) SaveBatch(_ context.Context, measurements []entities.RawMeasurement,
) (entities.SaveBatchResult, error) {
	*r.calls = append(*r.calls, "SaveBatch")
	if r.err != nil {
		return entities.SaveBatchResult{}, r.err
	}
	r.saved = measurements
	if r.allDuplicates {
		return entities.SaveBatchResult{}, nil
	}
	return entities.SaveBatchResult{Inserted: len(measurements)}, nil
}

type fakeIngestionStateRepository struct {
	calls    *[]string
	batchSeq int64
	created  bool
	latest   *entities.IngestionState
	saved    *entities.IngestionState
}

func (r *fakeIngestionStateRepository) FindLatestByMeasuringPointID(context.Context, int64) (*entities.IngestionState, error) {
	return r.latest, nil
}

func (r *fakeIngestionStateRepository) Save(_ context.Context, state *entities.IngestionState) (entities.SaveStateResult, error) {
	*r.calls = append(*r.calls, "SaveState")
	r.saved = state
	return entities.SaveStateResult{BatchSeq: r.batchSeq, LastIngestedAt: *state.LastIngestedAt}, nil
}

func (r *fakeIngestionStateRepository) SaveIfAbsent(_ context.Context, state *entities.IngestionState,
) (entities.SaveStateResult, bool, error) {
	*r.calls = append(*r.calls, "SaveIfAbsent")
	if !r.created {
		return entities.SaveStateResult{}, false, nil
	}
	r.saved = state
	return entities.SaveStateResult{BatchSeq: r.batchSeq, LastIngestedAt: *state.LastIngestedAt}, true, nil
}

type fakeDeviceRepository struct{}

func (fakeDeviceRepository) FindByMeasuringPointID(_ context.Context, measuringPointID int64) (*entities.Device, error) {
	return &entities.Device{MeasuringPointID: measuringPointID}, nil
}

func (fakeDeviceRepository) FindActive(context.Context) ([]entities.Device, error) {
	return nil, nil
}

type fakeDataProvider struct {
	measurements []entities.RawMeasurement
}

func (p fakeDataProvider) GetMeasurements(context.Context, *entities.Device, time.Time, time.Time) ([]entities.RawMeasurement, error) {
	return p.measurements, nil
}

func assertBatchID(t *testing.T, measurements []entities.RawMeasurement, want int64) {
	t.Helper()
	if len(measurements) == 0 {
		t.Fatal("no se guardaron mediciones")
	}
	for i, m := range measurements {
		if m.IngestionBatchID == nil || *m.IngestionBatchID != want {
			t.Fatalf("medición %d: ingestion_batch_id = %v, se esperaba %d", i, m.IngestionBatchID, want)
		}
	}
}
