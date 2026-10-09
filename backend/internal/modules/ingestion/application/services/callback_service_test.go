package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// fakeCallbackDevices permite simular punto inexistente o fallo de consulta.
type fakeCallbackDevices struct {
	fakeDeviceRepository
	missing bool
	err     error
}

func (f *fakeCallbackDevices) FindByMeasuringPointID(ctx context.Context, id int64) (*entities.Device, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.missing {
		return nil, nil
	}
	return f.fakeDeviceRepository.FindByMeasuringPointID(ctx, id)
}

type callbackFixture struct {
	service      *CallbackService
	devices      *fakeCallbackDevices
	measurements *fakeMeasurementRepository
	states       *fakeStateRepository
	uow          *fakeUnitOfWork
}

func newCallbackFixture(t *testing.T) *callbackFixture {
	t.Helper()
	f := &callbackFixture{
		devices:      &fakeCallbackDevices{},
		measurements: &fakeMeasurementRepository{},
		states:       &fakeStateRepository{nextID: 7},
	}
	f.uow = &fakeUnitOfWork{measurements: f.measurements, states: f.states, seq: &sequence{}}
	f.service = NewCallbackService(
		f.devices,
		f.uow,
		NewCallbackValidator(bogotaLocation(t)),
		logger.New("test", "Info"),
	)
	return f
}

func TestCallbackService_StoresNewRecord(t *testing.T) {
	f := newCallbackFixture(t)

	response, err := f.service.Receive(context.Background(), validCallback())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	want := dtos.CallbackResponse{Status: dtos.CallbackStatusStored, RecordID: "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f", ReceivedRecords: 3}
	if response != want {
		t.Fatalf("respuesta = %+v, se esperaba %+v", response, want)
	}

	if len(f.states.saved) != 1 {
		t.Fatalf("se guardaron %d estados, se esperaba 1", len(f.states.saved))
	}
	state := f.states.saved[0]
	if state.RecordID == nil || *state.RecordID != want.RecordID {
		t.Fatalf("record_id no persistido: %+v", state)
	}
	if state.Source != CallbackSource || state.MeasuringPointID != 11556 || state.ReceivedRecords != 3 {
		t.Fatalf("estado mal construido: %+v", state)
	}
	if wantTS := time.Date(2026, 10, 8, 16, 15, 41, 0, bogotaLocation(t)); state.LastIngestedAt == nil || !state.LastIngestedAt.Equal(wantTS) {
		t.Fatalf("last_ingested_at = %v, se esperaba %v", state.LastIngestedAt, wantTS)
	}

	// Sin mapeo de métricas: el callback todavía no escribe raw_measurements.
	if len(f.measurements.saved) != 0 {
		t.Fatalf("no se esperaban mediciones guardadas, hay %d", len(f.measurements.saved))
	}
}

func TestCallbackService_RetryIsAcknowledgedAsDuplicate(t *testing.T) {
	f := newCallbackFixture(t)

	if _, err := f.service.Receive(context.Background(), validCallback()); err != nil {
		t.Fatalf("primer envío: %v", err)
	}
	response, err := f.service.Receive(context.Background(), validCallback())
	if err != nil {
		t.Fatalf("reintento: %v", err)
	}

	if response.Status != dtos.CallbackStatusDuplicate {
		t.Fatalf("status = %q, se esperaba duplicate", response.Status)
	}
	if len(f.states.saved) != 1 {
		t.Fatalf("el reintento duplicó el registro: %d filas", len(f.states.saved))
	}
}

func TestCallbackService_LogsReceivedVariables(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	f := newCallbackFixture(t)
	if _, err := f.service.Receive(context.Background(), validCallback()); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"env_temperature_c", "22.4", "env_humidity_pct", "77.01", "accel_drift_deg_cloud", "AGATHA02"} {
		if !strings.Contains(out, want) {
			t.Fatalf("el log no muestra %q:\n%s", want, out)
		}
	}
}

func TestCallbackService_InvalidPayloadIsNotPersisted(t *testing.T) {
	f := newCallbackFixture(t)
	payload := validCallback()
	payload.Event = "otro"

	_, err := f.service.Receive(context.Background(), payload)

	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != appErrors.CodeInvalidInput {
		t.Fatalf("se esperaba INVALID_INPUT, llegó %v", err)
	}
	if len(f.states.saved) != 0 {
		t.Fatal("un payload inválido no debe persistirse")
	}
}

func TestCallbackService_UnknownMeasuringPoint(t *testing.T) {
	f := newCallbackFixture(t)
	f.devices.missing = true

	_, err := f.service.Receive(context.Background(), validCallback())

	if !errors.Is(err, appErrors.ErrDeviceNotFound) {
		t.Fatalf("se esperaba ErrDeviceNotFound, llegó %v", err)
	}
	if len(f.states.saved) != 0 {
		t.Fatal("no debe persistirse un registro de un punto inexistente")
	}
}

func TestCallbackService_DeviceLookupFailureIsRetryable(t *testing.T) {
	f := newCallbackFixture(t)
	f.devices.err = errors.New("conexión rechazada")

	_, err := f.service.Receive(context.Background(), validCallback())

	if !errors.Is(err, ingestionErrors.ErrConsultingDevice) {
		t.Fatalf("se esperaba ErrConsultingDevice, llegó %v", err)
	}
}

func TestCallbackService_PersistenceFailureIsRetryable(t *testing.T) {
	f := newCallbackFixture(t)
	f.states.err = errors.New("timeout")

	_, err := f.service.Receive(context.Background(), validCallback())

	if !errors.Is(err, ingestionErrors.ErrPersistingIngestionState) {
		t.Fatalf("se esperaba ErrPersistingIngestionState, llegó %v", err)
	}
}

func TestCallbackService_CommitFailureIsRetryable(t *testing.T) {
	f := newCallbackFixture(t)
	f.uow.commitErr = errors.New("commit falló")

	_, err := f.service.Receive(context.Background(), validCallback())

	if !errors.Is(err, ingestionErrors.ErrPersistingIngestionState) {
		t.Fatalf("se esperaba ErrPersistingIngestionState, llegó %v", err)
	}
	if len(f.states.saved) != 0 {
		t.Fatal("el rollback debe descartar el estado")
	}
}
