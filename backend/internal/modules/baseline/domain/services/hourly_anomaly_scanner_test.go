package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// Rango del barrido: t ∈ [from + 24 h, to]. La ventana que termina en
// from + 24 h es [from, from + 24 h): está completa dentro del rango.
func TestHourlyAnomalyScannerRange(t *testing.T) {
	samples, sig, stats, _, from, to := scanFixture(t)

	// Con corte 0 toda hora con I calculable queda marcada: marks = horas evaluadas.
	cfg := DefaultConfig()
	cfg.ExclusionICutoff = 0
	marks := NewHourlyAnomalyScanner(cfg).Scan(samples, sig, stats, from, to)
	if len(marks) == 0 {
		t.Fatal("no se evaluó ninguna hora")
	}

	t.Run("la primera hora es from + 24 h", func(t *testing.T) {
		if want := from.Add(24 * time.Hour); !marks[0].T.Equal(want) {
			t.Fatalf("primera hora = %v, want %v", marks[0].T, want)
		}
	})

	t.Run("la última hora es to", func(t *testing.T) {
		if last := marks[len(marks)-1].T; !last.Equal(to) {
			t.Fatalf("última hora = %v, want %v", last, to)
		}
	})

	t.Run("90 días alineados ⇒ 24 × 90 − 23 horas", func(t *testing.T) {
		from90, to90 := day0, day0.AddDate(0, 0, 90)
		hours := 0
		eachScanWindow(nil, from90, to90, func(time.Time, []features.Sample) { hours++ })
		if want := 24*90 - 23; hours != want {
			t.Fatalf("horas evaluadas = %d, want %d", hours, want)
		}
	})

	t.Run("la primera ventana incluye el sample en from y ninguno anterior", func(t *testing.T) {
		before := features.Sample{T: from.Add(-features.Slot), B2: 50, HasB2: true}
		withEarlier := append([]features.Sample{before}, samples...)

		calls := 0
		eachScanWindow(withEarlier, from, to, func(end time.Time, window []features.Sample) {
			calls++
			if calls > 1 {
				return
			}
			if !end.Equal(from.Add(24 * time.Hour)) {
				t.Fatalf("primera ventana termina en %v", end)
			}
			if len(window) == 0 || !window[0].T.Equal(from) {
				t.Fatalf("primer sample de la ventana = %v, want %v", window, from)
			}
			for _, s := range window {
				if s.T.Before(from) {
					t.Fatalf("sample anterior a from en la ventana: %v", s.T)
				}
			}
			if len(window) != features.ExpectedSamples(24) {
				t.Fatalf("la ventana tiene %d samples, want %d", len(window), features.ExpectedSamples(24))
			}
		})
		if calls == 0 {
			t.Fatal("no se recorrió ninguna ventana")
		}
	})
}
