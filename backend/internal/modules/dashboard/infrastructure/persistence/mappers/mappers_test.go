package mappers

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"
)

func TestToDeviceSummary(t *testing.T) {
	seen := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	battery := 3.7

	got := ToDeviceSummary(models.DeviceSummaryModel{
		MeasuringPointID: 11556, Name: "PK 10+200", LastConnection: &seen, LastBatteryLevel: &battery,
	})

	if got.ID != 11556 || got.Name != "PK 10+200" || got.LastConnection != &seen || got.LastBatteryLevel != &battery {
		t.Fatalf("resumen mal mapeado: %+v", got)
	}

	empty := ToDeviceSummary(models.DeviceSummaryModel{MeasuringPointID: 1})
	if empty.LastConnection != nil || empty.LastBatteryLevel != nil {
		t.Fatalf("sin conexión ni batería deben quedar nil: %+v", empty)
	}
}

func TestToRawMeasurement(t *testing.T) {
	ts := time.Date(2026, 11, 1, 10, 15, 0, 0, time.UTC)

	got := ToRawMeasurement(models.RawMeasurementModel{MetricID: 13, Timestamp: ts, MetricName: "B2", Unit: "-", Value: 51.5})

	if got.MetricID != 13 || !got.Timestamp.Equal(ts) || got.MetricName != "B2" || got.Unit != "-" || got.Value != 51.5 {
		t.Fatalf("medición mal mapeada: %+v", got)
	}
}
