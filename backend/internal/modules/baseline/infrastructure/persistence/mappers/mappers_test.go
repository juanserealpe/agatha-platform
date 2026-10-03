package mappers

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

var ids = MetricIDs{B1: 12, B2: 13, Planitud: 18}

func TestToSamplesPivot(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(15 * time.Minute)

	rows := []models.RawSampleRowModel{
		{Timestamp: t0, MetricID: 12, Value: 1},
		{Timestamp: t0, MetricID: 13, Value: 50},
		{Timestamp: t0, MetricID: 18, Value: 0.4},
		{Timestamp: t1, MetricID: 13, Value: math.NaN()}, // B2 no finito
		{Timestamp: t1, MetricID: 18, Value: 0.5},        // falta B1
	}

	out := ToSamples(rows, ids)
	if len(out) != 2 {
		t.Fatalf("len = %d, se esperaban 2", len(out))
	}
	if !out[0].HasB1 || !out[0].HasB2 || !out[0].HasPl || out[0].B2 != 50 {
		t.Fatalf("out[0] = %+v", out[0])
	}
	if out[1].HasB1 || out[1].HasB2 || !out[1].HasPl || out[1].Pl != 0.5 {
		t.Fatalf("out[1] = %+v", out[1])
	}
}

func TestTextArray(t *testing.T) {
	v, _ := models.TextArray{"A", "B_C"}.Value()
	if v != "{A,B_C}" {
		t.Fatalf("Value = %v", v)
	}

	var a models.TextArray
	for _, src := range []any{"{A,B_C}", []byte(`{"A","B_C"}`)} {
		if err := a.Scan(src); err != nil || !reflect.DeepEqual(a, models.TextArray{"A", "B_C"}) {
			t.Fatalf("Scan(%v) = %v, %v", src, a, err)
		}
	}
	if err := a.Scan("{}"); err != nil || len(a) != 0 {
		t.Fatalf("Scan({}) = %v", a)
	}
}

func TestBaselineVersionRoundTrip(t *testing.T) {
	p90 := 0.7
	minEl := 68.0
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	v := entities.BaselineVersion{
		MeasuringPointID: 11556,
		ValidFrom:        from.AddDate(0, 0, 90),
		HistoryFrom:      from,
		HistoryTo:        from.AddDate(0, 0, 90),
		Signal:           entities.SignalBaseline{MedianB2: 54, ThetaD: 67, P90Planitud: &p90, MinElevatedB2: &minEl, DeltaMin: 1, NRecords: 8000, NPlanitudRecords: 7990},
		Components: []entities.ComponentStat{
			{WindowHours: 24, Component: "C", Median: 0.2, MAD: 0.05, NWindows: 300},
			{WindowHours: 6, Component: "EX", Median: 30, MAD: 10, NWindows: 600},
			{WindowHours: 6, Component: "D", Median: 8, MAD: 2, NWindows: 600},
		},
		Confidence:       entities.ConfidenceNormal,
		ExcludedPeriods:  []entities.ExcludedPeriod{{From: from.AddDate(0, 0, 10), To: from.AddDate(0, 0, 11), PeakI: 0.99}},
		Estimation:       entities.EstimationSummary{Passes: 4, Converged: true, ExcludedRatio: 0.218},
		Flags:            []entities.Flag{entities.FlagSignalLowVariability},
		Trigger:          entities.TriggerScheduled,
		AlgorithmVersion: "1.0.0",
		CreatedAt:        from.AddDate(0, 0, 90),
	}

	m, err := ToBaselineVersionModel(v)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ToBaselineVersion(m)
	if err != nil {
		t.Fatal(err)
	}

	// El mapper ordena los componentes: 6 h (D, EX) y luego 24 h (C).
	if back.Components[0].Component != "D" || back.Components[1].Component != "EX" || back.Components[2].Component != "C" {
		t.Fatalf("orden = %+v", back.Components)
	}
	back.Components = v.Components
	if !reflect.DeepEqual(v, back) {
		t.Fatalf("round trip difiere:\n%+v\n%+v", v, back)
	}
}
