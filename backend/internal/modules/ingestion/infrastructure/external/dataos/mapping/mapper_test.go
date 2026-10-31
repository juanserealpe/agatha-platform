package mapping

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
)

func TestMapper_HalvesVoltageBeforePersisting(t *testing.T) {
	mapper := NewMapper(NewTimestampParser(time.UTC))
	device := &entities.Device{MeasuringPointID: 11556}
	series := dashboard.Series{
		Name:       "Voltaje (V)",
		DataPoints: []dashboard.DataPoint{{Timestamp: "2026-10-08T16:15:41Z", Value: 3700}},
	}

	measurements, err := mapper.MapSeries(device, series)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(measurements) != 1 || measurements[0].MetricID != 3 || measurements[0].Value != 1850 {
		t.Fatalf("voltaje debía llegar dividido entre 2: %+v", measurements)
	}
}
