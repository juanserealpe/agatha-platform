package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

func bogotaLocation(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// validCallback reproduce el ejemplo de Dataflow Labs.
func validCallback() dtos.DataRecordCallback {
	return dtos.DataRecordCallback{
		Event:     "data_record",
		RecordID:  "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f",
		MpID:      11556,
		SerialID:  "AGATHA02",
		Timestamp: "2026-10-08T16:15:41",
		Records: []dtos.CallbackReading{
			{DeviceID: 11556, Timestamp: "2026-10-08T16:15:41", Name: "env_temperature_c", Readings: "22.4"},
			{DeviceID: 11556, Timestamp: "2026-10-08T16:15:41", Name: "env_humidity_pct", Readings: "77.01"},
			{DeviceID: 11556, Timestamp: "2026-10-08T16:15:41", Name: "accel_drift_deg_cloud", Readings: "0.0"},
		},
	}
}

func TestCallbackValidator_ValidPayload(t *testing.T) {
	loc := bogotaLocation(t)
	record, err := NewCallbackValidator(loc).Validate(validCallback())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if record.RecordID != "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f" || record.MeasuringPointID != 11556 || record.SerialID != "AGATHA02" {
		t.Fatalf("cabecera mal copiada: %+v", record)
	}

	// Sin zona horaria ⇒ se interpreta en America/Bogota (misma convención que DataOS).
	want := time.Date(2026, 10, 8, 16, 15, 41, 0, loc)
	if !record.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, se esperaba %v", record.Timestamp, want)
	}

	if len(record.Readings) != 3 {
		t.Fatalf("se esperaban 3 variables, llegaron %d", len(record.Readings))
	}
	// El literal numérico se conserva tal cual (sin pasar por float64).
	if got := record.Readings[2]; got.Name != "accel_drift_deg_cloud" || got.Value != "0.0" || got.RawTimestamp != "2026-10-08T16:15:41" {
		t.Fatalf("variable mal copiada: %+v", got)
	}
	if !record.Readings[1].Timestamp.Equal(want) {
		t.Fatalf("timestamp de variable = %v, se esperaba %v", record.Readings[1].Timestamp, want)
	}
}

func TestCallbackValidator_AcceptsTimestampWithOffsetAndFraction(t *testing.T) {
	payload := validCallback()
	payload.Timestamp = "2026-10-08T21:15:41.250Z"
	payload.Records[0].Timestamp = "2026-10-08T16:15:41.5-05:00"

	record, err := NewCallbackValidator(bogotaLocation(t)).Validate(payload)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if want := time.Date(2026, 10, 8, 21, 15, 41, 250_000_000, time.UTC); !record.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, se esperaba %v", record.Timestamp, want)
	}
	if want := time.Date(2026, 10, 8, 21, 15, 41, 500_000_000, time.UTC); !record.Readings[0].Timestamp.Equal(want) {
		t.Fatalf("timestamp de variable = %v, se esperaba %v", record.Readings[0].Timestamp, want)
	}
}

func TestCallbackValidator_OptionalVariablesMayBeAbsent(t *testing.T) {
	payload := validCallback()
	payload.Records = payload.Records[:1] // sin humedad ni accel_drift_deg_cloud

	if _, err := NewCallbackValidator(bogotaLocation(t)).Validate(payload); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestCallbackValidator_RejectsInvalidPayloads(t *testing.T) {
	cases := map[string]struct {
		mutate func(*dtos.DataRecordCallback)
		detail string
	}{
		"evento distinto":       {func(p *dtos.DataRecordCallback) { p.Event = "data_records" }, "event"},
		"record_id vacío":       {func(p *dtos.DataRecordCallback) { p.RecordID = "  " }, "record_id"},
		"record_id muy largo":   {func(p *dtos.DataRecordCallback) { p.RecordID = strings.Repeat("a", 129) }, "record_id"},
		"mp_id ausente":         {func(p *dtos.DataRecordCallback) { p.MpID = 0 }, "mp_id"},
		"mp_id negativo":        {func(p *dtos.DataRecordCallback) { p.MpID = -1 }, "mp_id"},
		"serial_id vacío":       {func(p *dtos.DataRecordCallback) { p.SerialID = "" }, "serial_id"},
		"timestamp vacío":       {func(p *dtos.DataRecordCallback) { p.Timestamp = "" }, "timestamp"},
		"timestamp inválido":    {func(p *dtos.DataRecordCallback) { p.Timestamp = "2026-13-08T16:15:41" }, "timestamp"},
		"timestamp sin hora":    {func(p *dtos.DataRecordCallback) { p.Timestamp = "2026-10-08" }, "timestamp"},
		"records vacío":         {func(p *dtos.DataRecordCallback) { p.Records = nil }, "records"},
		"device_id distinto":    {func(p *dtos.DataRecordCallback) { p.Records[1].DeviceID = 999 }, "records[1].device_id"},
		"name vacío":            {func(p *dtos.DataRecordCallback) { p.Records[0].Name = "" }, "records[0].name"},
		"readings ausente":      {func(p *dtos.DataRecordCallback) { p.Records[2].Readings = "" }, "records[2].readings"},
		"timestamp de variable": {func(p *dtos.DataRecordCallback) { p.Records[0].Timestamp = "ayer" }, "records[0].timestamp"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			payload := validCallback()
			tc.mutate(&payload)

			_, err := NewCallbackValidator(bogotaLocation(t)).Validate(payload)

			var appErr *appErrors.AppError
			if !errors.As(err, &appErr) || appErr.Code != appErrors.CodeInvalidInput {
				t.Fatalf("se esperaba INVALID_INPUT, llegó %v", err)
			}
			if !strings.Contains(appErr.Message, tc.detail) {
				t.Fatalf("el mensaje %q no menciona %q", appErr.Message, tc.detail)
			}
		})
	}
}
