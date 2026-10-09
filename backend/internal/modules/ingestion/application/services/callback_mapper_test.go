package services

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

func callbackRecord(readings ...entities.CallbackReading) *entities.CallbackRecord {
	return &entities.CallbackRecord{
		RecordID:         "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f",
		MeasuringPointID: 11556,
		SerialID:         "AGATHA02",
		Timestamp:        time.Date(2026, 10, 8, 16, 15, 41, 0, time.UTC),
		Readings:         readings,
	}
}

func reading(name, value string, ts time.Time) entities.CallbackReading {
	return entities.CallbackReading{DeviceID: 11556, Name: name, Value: value, Timestamp: ts}
}

func TestCallbackMapper_MapsEveryKnownVariableToItsMetricID(t *testing.T) {
	want := map[string]uint64{
		"accel_pitch":           1,
		"accel_roll":            2,
		"power_vbat_mv":         3,
		"power_ibat_ma":         4,
		"power_battery_pct":     5,
		"piezo_rms":             6,
		"piezo_activity":        7,
		"accel_mag_avg":         8,
		"accel_activity_peak":   9,
		"pspec_ratio":           10,
		"pspec_b0_wind":         11,
		"pspec_b1_leak":         12,
		"pspec_b2_leak":         13,
		"pspec_b3_guard":        14,
		"accel_base_pitch":      15,
		"accel_base_roll":       16,
		"pspec_rms":             17,
		"pspec_flatness":        18,
		"accel_drift_deg_cloud": 19,
		"env_temperature_c":     20,
		"env_humidity_pct":      21,
	}
	if len(want) != len(callbackMetricIDMap) {
		t.Fatalf("el mapa tiene %d variables, se esperaban las %d del catálogo", len(callbackMetricIDMap), len(want))
	}

	ts := time.Date(2026, 10, 8, 16, 15, 41, 0, time.UTC)
	for name, metricID := range want {
		measurements, unmapped, err := NewCallbackMapper().Map(callbackRecord(reading(name, "1.5", ts)))
		if err != nil {
			t.Fatalf("%s: error inesperado: %v", name, err)
		}
		if len(unmapped) != 0 || len(measurements) != 1 {
			t.Fatalf("%s: mediciones=%d sin mapeo=%v", name, len(measurements), unmapped)
		}
		if measurements[0].MetricID != metricID {
			t.Fatalf("%s: metric_id = %d, se esperaba %d", name, measurements[0].MetricID, metricID)
		}
	}
}

func TestCallbackMapper_BuildsRawMeasurementFromReading(t *testing.T) {
	recordTS := time.Date(2026, 10, 8, 16, 15, 41, 0, time.UTC)
	readingTS := recordTS.Add(-30 * time.Second)

	measurements, _, err := NewCallbackMapper().Map(callbackRecord(reading("env_temperature_c", "22.4", readingTS)))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	want := entities.RawMeasurement{
		Timestamp:        readingTS, // el de la variable, no el del registro
		MeasuringPointID: 11556,
		MetricID:         20,
		Value:            22.4,
		Source:           CallbackSource,
	}
	if !reflect.DeepEqual(measurements, []entities.RawMeasurement{want}) {
		t.Fatalf("medición = %+v, se esperaba %+v", measurements, want)
	}
}

func TestCallbackMapper_ReportsUnmappedVariablesWithoutFailing(t *testing.T) {
	ts := time.Date(2026, 10, 8, 16, 15, 41, 0, time.UTC)

	measurements, unmapped, err := NewCallbackMapper().Map(callbackRecord(
		reading("env_humidity_pct", "77.01", ts),
		reading("piezo_peak", "812", ts),
		reading("accel_drift_alarm_cloud", "0", ts),
		reading("piezo_leak_suspected", "1", ts),
	))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(measurements) != 1 || measurements[0].MetricID != 21 {
		t.Fatalf("solo humedad debía mapearse: %+v", measurements)
	}
	if want := []string{"piezo_peak", "accel_drift_alarm_cloud", "piezo_leak_suspected"}; !reflect.DeepEqual(unmapped, want) {
		t.Fatalf("sin mapeo = %v, se esperaba %v", unmapped, want)
	}
}

// Variables reales del firmware sin métrica en el catálogo: se omiten.
func TestCallbackMapper_OmitsVariablesOutsideCatalog(t *testing.T) {
	ts := time.Date(2026, 10, 9, 9, 45, 42, 0, time.UTC)
	names := []string{
		"accel_activity_avg", "accel_movement", "accel_shock", "piezo_peak", "piezo_leak_suspected",
		"piezo_leak_confirmed", "power_energy_mwh", "power_modem_ma_avg", "power_modem_duty_pct",
		"power_p_in", "power_p_out", "samples", "accel_x_avg", "accel_z_std", "accel_drift_deg",
		"accel_vibration", "accel_event", "accel_drift_alarm", "accel_baseline_valid", "accel_drift_run",
		"accel_settle", "pspec_centroid", "pspec_leak_suspect", "accel_baseline_valid_cloud",
		"accel_drift_alarm_cloud", "accel_drift_run_cloud", "accel_settle_cloud",
	}
	readings := make([]entities.CallbackReading, 0, len(names))
	for _, name := range names {
		readings = append(readings, reading(name, "1", ts))
	}

	measurements, unmapped, err := NewCallbackMapper().Map(callbackRecord(readings...))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(measurements) != 0 {
		t.Fatalf("no se esperaban mediciones: %+v", measurements)
	}
	if !reflect.DeepEqual(unmapped, names) {
		t.Fatalf("sin mapeo = %v, se esperaba %v", unmapped, names)
	}
}

func TestCallbackMapper_RejectsValueOutOfFloatRange(t *testing.T) {
	ts := time.Date(2026, 10, 8, 16, 15, 41, 0, time.UTC)

	_, _, err := NewCallbackMapper().Map(callbackRecord(reading("piezo_rms", "1e400", ts)))

	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != appErrors.CodeInvalidInput {
		t.Fatalf("se esperaba INVALID_INPUT, llegó %v", err)
	}
}
