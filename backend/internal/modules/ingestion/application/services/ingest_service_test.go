package services

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// ---------------------------------------------------------
// Fakes
// ---------------------------------------------------------

// sequence registra el orden de los efectos observables (commit, publish).
type sequence struct{ steps []string }

func (s *sequence) add(step string) { s.steps = append(s.steps, step) }

type fakeDataProvider struct {
	measurements []entities.RawMeasurement
}

func (f *fakeDataProvider) GetMeasurements(context.Context, *entities.Device, time.Time, time.Time) ([]entities.RawMeasurement, error) {
	return f.measurements, nil
}

type fakeDeviceRepository struct{}

func (f *fakeDeviceRepository) Save(context.Context, entities.Device) error { return nil }

func (f *fakeDeviceRepository) FindByMeasuringPointID(_ context.Context, id int64) (*entities.Device, error) {
	return &entities.Device{MeasuringPointID: id, Active: true}, nil
}

func (f *fakeDeviceRepository) List(context.Context) ([]entities.Device, error) { return nil, nil }

type fakeMeasurementRepository struct {
	result entities.SaveBatchResult
	err    error
	saved  []entities.RawMeasurement
}

func (f *fakeMeasurementRepository) Save(context.Context, entities.RawMeasurement) error { return nil }

func (f *fakeMeasurementRepository) SaveBatch(_ context.Context, m []entities.RawMeasurement) (entities.SaveBatchResult, error) {
	f.saved = append(f.saved, m...)
	return f.result, f.err
}

type fakeStateRepository struct {
	result entities.SaveStateResult
	err    error
	saved  []entities.IngestionState
}

func (f *fakeStateRepository) FindLatestByMeasuringPointID(context.Context, int64) (*entities.IngestionState, error) {
	return nil, nil
}

func (f *fakeStateRepository) Save(_ context.Context, state *entities.IngestionState) (entities.SaveStateResult, error) {
	f.saved = append(f.saved, *state)
	return f.result, f.err
}

func (f *fakeStateRepository) Update(context.Context, *entities.IngestionState) error { return nil }

// fakeUnitOfWork ejecuta fn con los repos fake y simula commit/rollback.
type fakeUnitOfWork struct {
	measurements *fakeMeasurementRepository
	states       *fakeStateRepository
	commitErr    error
	seq          *sequence
}

func (f *fakeUnitOfWork) Execute(
	ctx context.Context,
	fn func(context.Context, repositories.MeasurementRepository, repositories.IngestionStateRepository, repositories.IngestionGapRepository) error,
) error {
	if err := fn(ctx, f.measurements, f.states, nil); err != nil {
		f.seq.add("rollback")
		return err
	}
	if f.commitErr != nil {
		f.seq.add("rollback")
		return f.commitErr
	}
	f.seq.add("commit")
	return nil
}

type fakeEventPublisher struct {
	events []contract.MeasurementsIngested
	err    error
	seq    *sequence
}

func (f *fakeEventPublisher) PublishMeasurementsIngested(_ context.Context, e contract.MeasurementsIngested) error {
	f.seq.add("publish")
	f.events = append(f.events, e)
	return f.err
}

// ---------------------------------------------------------
// Fixture
// ---------------------------------------------------------

const measuringPointID int64 = 11556

var bogota = time.FixedZone("America/Bogota", -5*60*60)

type fixture struct {
	service      *IngestService
	provider     *fakeDataProvider
	measurements *fakeMeasurementRepository
	states       *fakeStateRepository
	uow          *fakeUnitOfWork
	publisher    *fakeEventPublisher
	seq          *sequence
	fixedNow     time.Time
}

// newFixture arma el servicio con fakes. withPublisher=false inyecta un publisher nil.
func newFixture(withPublisher bool) *fixture {
	seq := &sequence{}
	f := &fixture{
		provider: &fakeDataProvider{measurements: []entities.RawMeasurement{
			// Desordenadas y en hora de Bogotá: el evento debe usar min/max en UTC.
			{Timestamp: time.Date(2026, 9, 28, 10, 5, 0, 0, bogota), MeasuringPointID: measuringPointID, MetricID: 20, Value: 21.5, Source: "dataos"},
			{Timestamp: time.Date(2026, 9, 28, 10, 0, 0, 0, bogota), MeasuringPointID: measuringPointID, MetricID: 21, Value: 81.2, Source: "dataos"},
			{Timestamp: time.Date(2026, 9, 28, 10, 10, 0, 0, bogota), MeasuringPointID: measuringPointID, MetricID: 5, Value: 97, Source: "dataos"},
		}},
		measurements: &fakeMeasurementRepository{result: entities.SaveBatchResult{Inserted: 3}},
		states: &fakeStateRepository{result: entities.SaveStateResult{
			BatchSeq:       42,
			LastIngestedAt: time.Date(2026, 9, 28, 10, 10, 0, 0, bogota),
		}},
		seq:      seq,
		fixedNow: time.Date(2026, 9, 29, 8, 0, 0, 0, bogota),
	}
	f.uow = &fakeUnitOfWork{measurements: f.measurements, states: f.states, seq: seq}

	var publisher *fakeEventPublisher
	log := logger.New("ingestion-test", "INFO")
	if withPublisher {
		publisher = &fakeEventPublisher{seq: seq}
		f.publisher = publisher
		f.service = NewIngestService(f.provider, f.states, &fakeDeviceRepository{}, NewProcessingService(log),
			f.uow, log, NewIngestionValidator(log), publisher)
	} else {
		f.service = NewIngestService(f.provider, f.states, &fakeDeviceRepository{}, NewProcessingService(log),
			f.uow, log, NewIngestionValidator(log), nil)
	}
	f.service.now = func() time.Time { return f.fixedNow }

	return f
}

func (f *fixture) ingest() error {
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, bogota)
	end := time.Date(2026, 9, 28, 23, 59, 59, 0, bogota)
	return f.service.IngestDataOs(context.Background(), measuringPointID, start, end)
}

// ---------------------------------------------------------
// Tests
// ---------------------------------------------------------

func TestIngestDataOs_PublishesOnceAfterCommitWithExpectedFields(t *testing.T) {
	f := newFixture(true)

	if err := f.ingest(); err != nil {
		t.Fatalf("IngestDataOs devolvió %v", err)
	}

	if len(f.publisher.events) != 1 {
		t.Fatalf("se publicaron %d eventos, se esperaba 1", len(f.publisher.events))
	}
	want := contract.MeasurementsIngested{
		MeasuringPointID: measuringPointID,
		Source:           "dataos",
		From:             time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC),
		To:               time.Date(2026, 9, 28, 15, 10, 0, 0, time.UTC),
		Watermark:        time.Date(2026, 9, 28, 15, 10, 0, 0, time.UTC),
		BatchSeq:         42,
		InsertedRecords:  3,
		OccurredAt:       time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC),
		SchemaVersion:    contract.SchemaVersion,
	}
	got := f.publisher.events[0]
	if got != want {
		t.Fatalf("evento publicado:\n got  %+v\n want %+v", got, want)
	}
	for name, ts := range map[string]time.Time{"From": got.From, "To": got.To, "Watermark": got.Watermark, "OccurredAt": got.OccurredAt} {
		if ts.Location() != time.UTC {
			t.Errorf("%s no está en UTC: %v", name, ts.Location())
		}
	}
}

func TestIngestDataOs_PublishesAfterCommit(t *testing.T) {
	f := newFixture(true)

	if err := f.ingest(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(f.seq.steps, []string{"commit", "publish"}) {
		t.Fatalf("secuencia = %v, se esperaba [commit publish]", f.seq.steps)
	}
}

func TestIngestDataOs_DoesNotPublishWhenTransactionFails(t *testing.T) {
	cases := map[string]func(*fixture){
		"falla SaveBatch":   func(f *fixture) { f.measurements.err = errors.New("insert falló") },
		"falla Save estado": func(f *fixture) { f.states.err = errors.New("insert estado falló") },
		"falla el commit":   func(f *fixture) { f.uow.commitErr = errors.New("commit falló") },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(true)
			arrange(f)

			if err := f.ingest(); err == nil {
				t.Fatal("se esperaba error de la ingesta")
			}
			if len(f.publisher.events) != 0 {
				t.Fatalf("no debía publicar tras rollback: %+v", f.publisher.events)
			}
			if !slices.Equal(f.seq.steps, []string{"rollback"}) {
				t.Fatalf("secuencia = %v", f.seq.steps)
			}
		})
	}
}

func TestIngestDataOs_DoesNotPublishWhenNothingWasInserted(t *testing.T) {
	f := newFixture(true)
	f.measurements.result = entities.SaveBatchResult{Inserted: 0}

	if err := f.ingest(); err != nil {
		t.Fatalf("IngestDataOs devolvió %v", err)
	}
	if len(f.publisher.events) != 0 {
		t.Fatalf("no debía publicar con Inserted == 0: %+v", f.publisher.events)
	}
}

func TestIngestDataOs_DoesNotPublishWhenBatchIsEmpty(t *testing.T) {
	f := newFixture(true)
	f.provider.measurements = nil

	if err := f.ingest(); !errors.Is(err, ingestionErrors.ErrRecivingIngestedData) {
		t.Fatalf("error = %v, se esperaba ErrRecivingIngestedData", err)
	}
	if len(f.publisher.events) != 0 || len(f.seq.steps) != 0 {
		t.Fatalf("no debía persistir ni publicar: %v", f.seq.steps)
	}
}

func TestIngestDataOs_PublisherErrorDoesNotFailIngestion(t *testing.T) {
	f := newFixture(true)
	f.publisher.err = errors.New("bus caído")

	if err := f.ingest(); err != nil {
		t.Fatalf("un error del publisher no debe fallar la ingesta: %v", err)
	}
	if !slices.Equal(f.seq.steps, []string{"commit", "publish"}) {
		t.Fatalf("secuencia = %v", f.seq.steps)
	}
}

func TestIngestDataOs_NilPublisherBehavesLikeBefore(t *testing.T) {
	withPublisher := newFixture(true)
	withoutPublisher := newFixture(false)

	errWith := withPublisher.ingest()
	errWithout := withoutPublisher.ingest()

	if errWith != nil || errWithout != nil {
		t.Fatalf("errores: con publisher %v, sin publisher %v", errWith, errWithout)
	}
	if !slices.Equal(withoutPublisher.seq.steps, []string{"commit"}) {
		t.Fatalf("secuencia sin publisher = %v", withoutPublisher.seq.steps)
	}
	// DeepEqual sigue punteros (LastIngestedAt es *time.Time).
	if !reflect.DeepEqual(withPublisher.measurements.saved, withoutPublisher.measurements.saved) ||
		!reflect.DeepEqual(withPublisher.states.saved, withoutPublisher.states.saved) {
		t.Fatal("la persistencia debe ser idéntica con y sin publisher")
	}
}
