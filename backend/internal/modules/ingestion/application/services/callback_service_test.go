package services

import (
	"context"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func newTestCallbackService(uow *fakeUnitOfWork) *CallbackService {
	return NewCallbackService(fakeDeviceRepository{}, uow, NewCallbackValidator(time.UTC), NewCallbackMapper(), logger.New("test", "Info"))
}

func callbackRequest() dtos.DataRecordCallback {
	return dtos.DataRecordCallback{
		Event:     "data_record",
		RecordID:  "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f",
		MpID:      11556,
		SerialID:  "AGATHA02",
		Timestamp: "2026-10-29T10:00:00",
		Records: []dtos.CallbackReading{
			{DeviceID: 11556, Timestamp: "2026-10-29T10:00:00", Name: "env_temperature_c", Readings: "22.4"},
			{DeviceID: 11556, Timestamp: "2026-10-29T10:00:00", Name: "env_humidity_pct", Readings: "77.01"},
		},
	}
}

func TestCallbackService_LinksMeasurementsToTheirReceipt(t *testing.T) {
	uow := newFakeUnitOfWork(42)

	response, err := newTestCallbackService(uow).Receive(context.Background(), callbackRequest())
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != dtos.CallbackStatusStored {
		t.Fatalf("status = %q", response.Status)
	}
	if got := uow.calls(); len(got) != 2 || got[0] != "SaveIfAbsent" || got[1] != "SaveBatch" {
		t.Fatalf("orden de escritura = %v, se esperaba [SaveIfAbsent SaveBatch]", got)
	}
	assertBatchID(t, uow.measurements.saved, 42)
}

func TestCallbackService_DuplicateWritesNoMeasurements(t *testing.T) {
	uow := newFakeUnitOfWork(42)
	uow.states.created = false

	response, err := newTestCallbackService(uow).Receive(context.Background(), callbackRequest())
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != dtos.CallbackStatusDuplicate {
		t.Fatalf("status = %q, se esperaba duplicate", response.Status)
	}
	if got := uow.calls(); len(got) != 1 {
		t.Fatalf("escrituras = %v, se esperaba solo SaveIfAbsent", got)
	}
}

func TestCallbackService_AllMeasurementsAlreadyStoredIsStillStored(t *testing.T) {
	uow := newFakeUnitOfWork(42)
	uow.measurements.allDuplicates = true

	response, err := newTestCallbackService(uow).Receive(context.Background(), callbackRequest())
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != dtos.CallbackStatusStored {
		t.Fatalf("status = %q: el recibo es nuevo aunque las mediciones no lo sean", response.Status)
	}
}
