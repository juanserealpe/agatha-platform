package knownevents

import (
	"context"
	"testing"
	"time"
)

func utc(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }

func TestStaticProvider(t *testing.T) {
	provider := NewStaticProvider()
	ctx := context.Background()
	all := utc(2026, 1, 1, 0)
	end := utc(2027, 1, 1, 0)

	t.Run("punto 11556: eventos del estudio en UTC y ordenados", func(t *testing.T) {
		events, err := provider.GetKnownEvents(ctx, 11556, all, end)
		if err != nil || len(events) != 7 {
			t.Fatalf("eventos = %d, err = %v", len(events), err)
		}

		// Las fechas del estudio son hora de Bogotá (UTC − 5).
		first, last := events[0], events[len(events)-1]
		if !first.From.Equal(utc(2026, 7, 23, 11)) || first.Label == "" {
			t.Fatalf("primer evento = %+v", first)
		}
		if !last.To.Equal(utc(2026, 8, 30, 5)) {
			t.Fatalf("último evento = %+v", last)
		}

		for i, e := range events {
			if !e.From.Before(e.To) || e.From.Location() != time.UTC {
				t.Fatalf("evento %d inválido: %+v", i, e)
			}
			if i > 0 && e.From.Before(events[i-1].From) {
				t.Fatalf("evento %d fuera de orden", i)
			}
		}
	})

	tests := []struct {
		name     string
		point    int64
		from, to time.Time
		want     int
	}{
		{"punto sin eventos", 999, all, end, 0},
		{"rango anterior a los eventos", 11556, utc(2026, 6, 1, 0), utc(2026, 7, 23, 11), 0},
		{"rango posterior a los eventos", 11556, utc(2026, 8, 30, 5), end, 0},
		{"rango que solo toca julio", 11556, utc(2026, 7, 1, 0), utc(2026, 8, 10, 0), 3},
		{"rango que solo toca agosto", 11556, utc(2026, 8, 10, 0), end, 4},
		{"rango que corta un evento por la mitad", 11556, utc(2026, 8, 21, 0), utc(2026, 8, 22, 0), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := provider.GetKnownEvents(ctx, tt.point, tt.from, tt.to)
			if err != nil || len(events) != tt.want {
				t.Fatalf("eventos = %d (%+v), err = %v, se esperaban %d", len(events), events, err, tt.want)
			}
		})
	}
}
