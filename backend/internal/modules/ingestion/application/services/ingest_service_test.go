package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
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

// fakeStateRepository simula ingestion_logs: cada Save agrega una fila con id
// autoincremental (BatchSeq) y devuelve el last_ingested_at persistido.
type fakeStateRepository struct {
	err    error
	nextID int64
	saved  []entities.IngestionState
}

func (f *fakeStateRepository) FindLatestByMeasuringPointID(context.Context, int64) (*entities.IngestionState, error) {
	if len(f.saved) == 0 {
		return nil, nil
	}
	latest := f.saved[len(f.saved)-1]
	return &latest, nil
}

func (f *fakeStateRepository) Save(_ context.Context, state *entities.IngestionState) (entities.SaveStateResult, error) {
	if f.err != nil {
		return entities.SaveStateResult{}, f.err
	}
	f.nextID++
	row := *state
	row.ID = uint64(f.nextID)
	f.saved = append(f.saved, row)

	result := entities.SaveStateResult{BatchSeq: f.nextID}
	if state.LastIngestedAt != nil {
		result.LastIngestedAt = *state.LastIngestedAt
	}
	return result, nil
}

// SaveIfAbsent simula el índice único sobre record_id.
func (f *fakeStateRepository) SaveIfAbsent(ctx context.Context, state *entities.IngestionState) (entities.SaveStateResult, bool, error) {
	if f.err != nil {
		return entities.SaveStateResult{}, false, f.err
	}
	for _, row := range f.saved {
		if row.RecordID != nil && state.RecordID != nil && *row.RecordID == *state.RecordID {
			return entities.SaveStateResult{}, false, nil
		}
	}
	result, err := f.Save(ctx, state)
	return result, err == nil, err
}

func (f *fakeStateRepository) Update(context.Context, *entities.IngestionState) error { return nil }

// fakeUnitOfWork ejecuta fn con los repos fake y simula commit/rollback:
// en rollback descarta lo que fn escribió en mediciones y estado.
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
	savedMeasurements, savedStates, nextID := len(f.measurements.saved), len(f.states.saved), f.states.nextID
	rollback := func() {
		f.measurements.saved = f.measurements.saved[:savedMeasurements]
		f.states.saved = f.states.saved[:savedStates]
		f.states.nextID = nextID
		f.seq.add("rollback")
	}

	if err := fn(ctx, f.measurements, f.states, nil); err != nil {
		rollback()
		return err
	}
	if f.commitErr != nil {
		rollback()
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
		states:       &fakeStateRepository{nextID: 41},
		seq:          seq,
		fixedNow:     time.Date(2026, 9, 29, 8, 0, 0, 0, bogota),
	}
	f.uow = &fakeUnitOfWork{measurements: f.measurements, states: f.states, seq: seq}

	log := logger.New("ingestion-test", "INFO")
	if withPublisher {
		f.publisher = &fakeEventPublisher{seq: seq}
		f.service = NewIngestService(f.provider, f.states, &fakeDeviceRepository{}, NewProcessingService(log),
			f.uow, log, NewIngestionValidator(log), f.publisher)
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

// captureLog redirige la salida del logger compartido mientras corre fn.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(io.Discard)
	fn()
	return buf.String()
}

// ---------------------------------------------------------
// Tests
// ---------------------------------------------------------

func TestIngestDataOs_SavesStateWithLatestMeasurementTimestamp(t *testing.T) {
	f := newFixture(true)

	if err := f.ingest(); err != nil {
		t.Fatalf("IngestDataOs devolvió %v", err)
	}

	if len(f.states.saved) != 1 {
		t.Fatalf("se guardaron %d filas en ingestion_logs, se esperaba 1", len(f.states.saved))
	}
	state := f.states.saved[0]
	wantLast := time.Date(2026, 9, 28, 10, 10, 0, 0, bogota)
	if state.LastIngestedAt == nil || !state.LastIngestedAt.Equal(wantLast) {
		t.Fatalf("last_ingested_at = %v, se esperaba el MAX(timestamp) del lote %v", state.LastIngestedAt, wantLast)
	}
	if state.MeasuringPointID != uint64(measuringPointID) || state.Source != "dataos" || state.ReceivedRecords != 3 {
		t.Fatalf("estado guardado = %+v", state)
	}
}

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
		BatchSeq:         int64(f.states.saved[0].ID), // id de la fila de ingestion_logs
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

func TestIngestDataOs_ConsecutiveBatchesHaveIncreasingBatchSeqAndWatermark(t *testing.T) {
	f := newFixture(true)

	if err := f.ingest(); err != nil {
		t.Fatal(err)
	}

	f.provider.measurements = []entities.RawMeasurement{
		{Timestamp: time.Date(2026, 9, 28, 10, 15, 0, 0, bogota), MeasuringPointID: measuringPointID, MetricID: 20, Value: 22, Source: "dataos"},
		{Timestamp: time.Date(2026, 9, 28, 10, 20, 0, 0, bogota), MeasuringPointID: measuringPointID, MetricID: 21, Value: 80, Source: "dataos"},
	}
	f.measurements.result = entities.SaveBatchResult{Inserted: 2}
	if err := f.ingest(); err != nil {
		t.Fatal(err)
	}

	if len(f.publisher.events) != 2 || len(f.states.saved) != 2 {
		t.Fatalf("eventos = %d, filas de estado = %d; se esperaban 2 y 2", len(f.publisher.events), len(f.states.saved))
	}
	first, second := f.publisher.events[0], f.publisher.events[1]
	if second.BatchSeq <= first.BatchSeq {
		t.Fatalf("batch_seq no es creciente: %d → %d", first.BatchSeq, second.BatchSeq)
	}
	if !second.Watermark.After(first.Watermark) {
		t.Fatalf("el watermark no avanzó: %v → %v", first.Watermark, second.Watermark)
	}
	latest, _ := f.states.FindLatestByMeasuringPointID(context.Background(), measuringPointID)
	if latest == nil || !latest.LastIngestedAt.Equal(time.Date(2026, 9, 28, 10, 20, 0, 0, bogota)) {
		t.Fatalf("último estado = %+v", latest)
	}
}

func TestIngestDataOs_StateSaveFailureRollsBackAndReturnsAppError(t *testing.T) {
	f := newFixture(true)
	f.states.err = errors.New("insert estado falló")

	err := f.ingest()

	if !errors.Is(err, ingestionErrors.ErrPersistingIngestionState) {
		t.Fatalf("error = %v, se esperaba ErrPersistingIngestionState", err)
	}
	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != ingestionErrors.CodePersistingIngestionState {
		t.Fatalf("el error no es un AppError con el código esperado: %#v", err)
	}
	if !slices.Equal(f.seq.steps, []string{"rollback"}) {
		t.Fatalf("secuencia = %v, se esperaba [rollback]", f.seq.steps)
	}
	if len(f.measurements.saved) != 0 || len(f.states.saved) != 0 {
		t.Fatalf("quedaron mediciones (%d) o estados (%d) tras el rollback", len(f.measurements.saved), len(f.states.saved))
	}
	if len(f.publisher.events) != 0 {
		t.Fatalf("no debía publicar tras rollback: %+v", f.publisher.events)
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

func TestIngestDataOs_NothingInsertedSavesStateButDoesNotPublish(t *testing.T) {
	f := newFixture(true)
	f.measurements.result = entities.SaveBatchResult{Inserted: 0}

	if err := f.ingest(); err != nil {
		t.Fatalf("IngestDataOs devolvió %v", err)
	}
	if len(f.publisher.events) != 0 {
		t.Fatalf("no debía publicar con Inserted == 0: %+v", f.publisher.events)
	}
	// Diseño original: el estado se guarda siempre que el lote procesado no esté vacío.
	if len(f.states.saved) != 1 || !slices.Equal(f.seq.steps, []string{"commit"}) {
		t.Fatalf("estados = %d, secuencia = %v; se esperaba 1 fila y [commit]", len(f.states.saved), f.seq.steps)
	}
}

func TestIngestDataOs_DoesNotPublishWhenBatchIsEmpty(t *testing.T) {
	f := newFixture(true)
	f.provider.measurements = nil

	if err := f.ingest(); !errors.Is(err, ingestionErrors.ErrRecivingIngestedData) {
		t.Fatalf("error = %v, se esperaba ErrRecivingIngestedData", err)
	}
	if len(f.publisher.events) != 0 || len(f.seq.steps) != 0 || len(f.states.saved) != 0 {
		t.Fatalf("no debía persistir ni publicar: %v", f.seq.steps)
	}
}

func TestIngestDataOs_PublisherErrorDoesNotFailIngestionAndLogsWarn(t *testing.T) {
	f := newFixture(true)
	f.publisher.err = errors.New("bus caído")

	var err error
	out := captureLog(t, func() { err = f.ingest() })

	if err != nil {
		t.Fatalf("un error del publisher no debe fallar la ingesta: %v", err)
	}
	if !slices.Equal(f.seq.steps, []string{"commit", "publish"}) {
		t.Fatalf("secuencia = %v", f.seq.steps)
	}
	if !strings.Contains(out, "[WARN]") || !strings.Contains(out, "no se pudo publicar el evento de mediciones ingeridas") {
		t.Fatalf("se esperaba un Warn por el fallo de publicación; log:\n%s", out)
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
