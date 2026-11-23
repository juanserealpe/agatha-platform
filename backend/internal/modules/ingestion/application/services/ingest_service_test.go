package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var dataOSStart = time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

func newTestIngestService(uow *fakeUnitOfWork) *IngestService {
	log := logger.New("test", "Info")
	measurements := []entities.RawMeasurement{
		{Timestamp: dataOSStart.Add(5 * time.Minute), MeasuringPointID: 11556, MetricID: 12, Value: 50, Source: "dataos"},
		{Timestamp: dataOSStart.Add(10 * time.Minute), MeasuringPointID: 11556, MetricID: 12, Value: 51, Source: "dataos"},
	}
	return NewIngestService(fakeDataProvider{measurements: measurements}, uow.states, fakeDeviceRepository{}, uow, log, NewIngestionValidator(log))
}

func TestIngestDataOs_SavesStateBeforeMeasurementsAndLinksThem(t *testing.T) {
	uow := newFakeUnitOfWork(7)

	if err := newTestIngestService(uow).IngestDataOs(context.Background(), 11556, dataOSStart, dataOSStart.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if got := uow.calls(); len(got) != 2 || got[0] != "SaveState" || got[1] != "SaveBatch" {
		t.Fatalf("orden de escritura = %v, se esperaba [SaveState SaveBatch]", got)
	}
	assertBatchID(t, uow.measurements.saved, 7)

	// received_records es el tamaño del lote pedido y last_ingested_at su MAX(timestamp).
	state := uow.states.saved
	if state.ReceivedRecords != 2 || !state.LastIngestedAt.Equal(dataOSStart.Add(10*time.Minute)) {
		t.Fatalf("estado = %+v", state)
	}
}

func TestIngestDataOs_NothingNewRollsBackTheBatch(t *testing.T) {
	// El catch-up vuelve a pedir rangos que el callback ya trajo: si no se
	// insertó ninguna medición, no debe quedar un lote vacío.
	uow := newFakeUnitOfWork(7)
	uow.measurements.allDuplicates = true

	err := newTestIngestService(uow).IngestDataOs(context.Background(), 11556, dataOSStart, dataOSStart.Add(15*time.Minute))
	if err != nil {
		t.Fatalf("sin mediciones nuevas no es un error: %v", err)
	}
	if uow.fnErr == nil {
		t.Fatal("la transacción debía revertirse para no dejar el lote vacío")
	}
}

func TestIngestDataOs_MeasurementFailureFailsTheIngestion(t *testing.T) {
	uow := newFakeUnitOfWork(7)
	uow.measurements.err = errors.New("db caída")

	err := newTestIngestService(uow).IngestDataOs(context.Background(), 11556, dataOSStart, dataOSStart.Add(15*time.Minute))
	if err == nil {
		t.Fatal("se esperaba error: el UoW debe revertir también el estado")
	}
}
