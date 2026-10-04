package services

import (
	"testing"
	"time"
)

var h0 = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

func hours(n int) time.Duration { return time.Duration(n) * time.Hour }

func TestSlotPlanner(t *testing.T) {
	planner := NewSlotPlanner(72)
	last := h0.Add(-hours(3))

	tests := []struct {
		name       string
		last       *time.Time
		readyUntil time.Time
		maxSlots   int
		wantFirst  time.Time
		wantLen    int
	}{
		{
			name:       "sin historial empieza 71 h antes de readyUntil (72 horas)",
			readyUntil: h0,
			maxSlots:   0,
			wantFirst:  h0.Add(-hours(71)),
			wantLen:    72,
		},
		{
			name:       "sin historial respeta el tope",
			readyUntil: h0,
			maxSlots:   24,
			wantFirst:  h0.Add(-hours(71)),
			wantLen:    24,
		},
		{
			name:       "con historial empieza en last + 1 h",
			last:       &last,
			readyUntil: h0,
			maxSlots:   24,
			wantFirst:  h0.Add(-hours(2)),
			wantLen:    3,
		},
		{
			name:       "con historial respeta el tope desde la primera pendiente",
			last:       &last,
			readyUntil: h0.Add(hours(100)),
			maxSlots:   72,
			wantFirst:  h0.Add(-hours(2)),
			wantLen:    72,
		},
		{
			name:       "readyUntil no alineado se trunca a la hora",
			last:       &last,
			readyUntil: h0.Add(59 * time.Minute),
			maxSlots:   0,
			wantFirst:  h0.Add(-hours(2)),
			wantLen:    3,
		},
		{
			name:       "readyUntil igual a last no planifica nada",
			last:       &last,
			readyUntil: last,
			maxSlots:   24,
			wantLen:    0,
		},
		{
			name:       "readyUntil anterior a last no planifica nada",
			last:       &last,
			readyUntil: last.Add(-hours(5)),
			maxSlots:   24,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planner.Plan(tt.last, tt.readyUntil, tt.maxSlots)

			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, se esperaba %d", len(got), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}
			if !got[0].Equal(tt.wantFirst) {
				t.Fatalf("primera hora = %v, se esperaba %v", got[0], tt.wantFirst)
			}
			for i := 1; i < len(got); i++ {
				if got[i].Sub(got[i-1]) != time.Hour {
					t.Fatalf("horas no consecutivas en %d: %v → %v", i, got[i-1], got[i])
				}
			}
			if last := got[len(got)-1]; last.After(tt.readyUntil) {
				t.Fatalf("última hora %v posterior a readyUntil %v", last, tt.readyUntil)
			}
		})
	}
}
