package bootstrap

import (
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

func f64(v float64) *float64 { return &v }

func effective(validFrom time.Time) baseline.EffectiveBaseline {
	first := validFrom.Add(-30 * 24 * time.Hour)
	return baseline.EffectiveBaseline{
		MeasuringPointID:  11556,
		Status:            baseline.StatusAvailable,
		VersionID:         12,
		Version:           3,
		Confidence:        "LOW",
		ValidFrom:         validFrom,
		DeviceFirstSeenAt: &first,
		Signal: &baseline.SignalStats{
			MedianB2: 53, ThetaD: 56, P90Planitud: f64(0.38), MinElevatedB2: f64(57), DeltaMin: 1,
		},
		Components: []baseline.ComponentStats{
			{WindowHours: 24, Component: "D", Median: 10, MAD: 3, NWindows: 300},
			{WindowHours: 24, Component: "P", Median: 0.5, MAD: 0.25, NWindows: 300},
			{WindowHours: 24, Component: "EX", Median: 100, MAD: 40, NWindows: 300},
			{WindowHours: 24, Component: "C", Median: 0.3, MAD: 0.05, NWindows: 299},
			{WindowHours: 6, Component: "D", Median: 9, MAD: 4, NWindows: 600},
			{WindowHours: 6, Component: "P", Median: 0.25, MAD: 0.25, NWindows: 600},
			{WindowHours: 6, Component: "EX", Median: 25, MAD: 10, NWindows: 600},
		},
		Flags:            []string{"SIGNAL_LOW_VARIABILITY"},
		AlgorithmVersion: "1.0.0",
	}
}

func TestToBaselineSnapshot(t *testing.T) {
	validFrom := time.Date(2026, 7, 28, 0, 10, 0, 0, time.UTC)
	snap := toBaselineSnapshot(effective(validFrom))

	if !snap.Available || snap.VersionID != 12 || snap.Version != 3 || snap.Confidence != "LOW" ||
		!snap.ValidFrom.Equal(validFrom) || snap.AlgorithmVersion != "1.0.0" || snap.FirstSeenAt == nil {
		t.Fatalf("metadatos = %+v", snap)
	}

	wantSignal := features.Signal{
		MedianB2: 53, ThetaD: 56, P90Planitud: 0.38, HasP90Planitud: true,
		MinElevatedB2: 57, HasMinElevated: true, DeltaMin: 1,
	}
	if snap.Signal != wantSignal {
		t.Fatalf("señal = %+v", snap.Signal)
	}

	s24, s6 := snap.Stats[24], snap.Stats[6]
	if s24.D != (features.Stat{Median: 10, MAD: 3, N: 300}) || s24.Ex.Median != 100 || !s24.HasC || s24.C.N != 299 {
		t.Fatalf("estadísticas 24 h = %+v", s24)
	}
	if s6.P != (features.Stat{Median: 0.25, MAD: 0.25, N: 600}) || s6.HasC {
		t.Fatalf("estadísticas 6 h = %+v", s6)
	}
	if len(snap.Flags) != 1 || snap.IsSignalDegenerate() {
		t.Fatalf("flags = %v", snap.Flags)
	}
}

func TestToBaselineSnapshotNilPointers(t *testing.T) {
	eb := effective(time.Now())
	eb.Signal.P90Planitud, eb.Signal.MinElevatedB2 = nil, nil

	snap := toBaselineSnapshot(eb)
	if snap.Signal.HasP90Planitud || snap.Signal.HasMinElevated || !snap.IsSignalDegenerate() {
		t.Fatalf("señal = %+v", snap.Signal)
	}
}

func TestToBaselineSnapshotNone(t *testing.T) {
	first := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := toBaselineSnapshot(baseline.EffectiveBaseline{
		Status: baseline.StatusNone, DeviceFirstSeenAt: &first, Components: []baseline.ComponentStats{}, Flags: []string{},
	})

	if snap.Available || snap.FirstSeenAt == nil || !snap.FirstSeenAt.Equal(first) || len(snap.Stats) != 0 {
		t.Fatalf("NONE = %+v", snap)
	}
}

func TestReadOnlyDSN(t *testing.T) {
	tests := []struct{ in, contains string }{
		{"postgres://u:p@localhost:5432/agatha?sslmode=disable", "default_transaction_read_only=on"},
		{"postgresql://u:p@localhost/agatha", "?default_transaction_read_only=on"},
		{"host=localhost user=u dbname=agatha", "dbname=agatha default_transaction_read_only=on"},
	}

	for _, tt := range tests {
		got := readOnlyDSN(tt.in)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("readOnlyDSN(%q) = %q", tt.in, got)
		}
	}

	if got := readOnlyDSN("postgres://u:p@h/db?sslmode=disable"); !strings.Contains(got, "sslmode=disable") {
		t.Errorf("perdió parámetros existentes: %q", got)
	}
}
