package entities

import "testing"

func TestNormalizeMeasurementValue_HalvesVoltage(t *testing.T) {
	if got := NormalizeMeasurementValue(VoltageMetricID, 3700); got != 1850 {
		t.Fatalf("voltaje = %v, se esperaba 1850", got)
	}
}

func TestNormalizeMeasurementValue_LeavesOtherMetricsUntouched(t *testing.T) {
	for _, metricID := range []uint64{1, 2, 4, 5, 20, 21} {
		if got := NormalizeMeasurementValue(metricID, 3700); got != 3700 {
			t.Fatalf("metric_id %d: valor = %v, se esperaba 3700", metricID, got)
		}
	}
}
