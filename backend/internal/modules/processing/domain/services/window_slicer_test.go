package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// series genera n samples cada 15 min terminando (exclusivo) en end, con B2 = b2.
func series(end time.Time, n int, b2 float64) []features.Sample {
	out := make([]features.Sample, n)
	start := end.Add(-time.Duration(n) * features.Slot)
	for i := range out {
		out[i] = features.Sample{T: start.Add(time.Duration(i) * features.Slot), B2: b2, HasB2: true}
	}
	return out
}

func TestSliceWindow(t *testing.T) {
	// 30 h de datos que terminan en h0 + 1 h: incluye un sample exactamente en h0.
	samples := series(h0.Add(time.Hour), 120, 50)

	w24 := SliceWindow(samples, h0, 24)
	if len(w24) != 96 {
		t.Fatalf("ventana 24 h = %d samples, se esperaban 96", len(w24))
	}
	if !w24[0].T.Equal(h0.Add(-hours(24))) {
		t.Fatalf("primer sample %v: el de t − 24 h debe estar incluido", w24[0].T)
	}
	if last := w24[len(w24)-1].T; !last.Before(h0) {
		t.Fatalf("último sample %v: el de t debe quedar excluido", last)
	}

	w6 := SliceWindow(samples, h0, 6)
	if len(w6) != 24 || !w6[0].T.Equal(h0.Add(-hours(6))) {
		t.Fatalf("ventana 6 h = %d samples desde %v", len(w6), w6[0].T)
	}

	if got := SliceWindow(samples, h0.Add(-hours(48)), 24); len(got) != 0 {
		t.Fatalf("ventana sin datos = %d samples, se esperaba vacía", len(got))
	}
	if got := SliceWindow(nil, h0, 24); len(got) != 0 {
		t.Fatalf("entrada vacía = %d samples", len(got))
	}
}
