package entities

import (
	"testing"
	"time"
)

func TestExclusionPeriod_KnownEvent(t *testing.T) {
	cot := time.FixedZone("COT", -5*3600)
	ended := h(5).In(cot)

	t.Run("finalizado: usa su fin", func(t *testing.T) {
		got := ExclusionPeriod{StartedAt: h(1).In(cot), EndedAt: &ended}.KnownEvent(h(100))
		if !got.From.Equal(h(1)) || !got.To.Equal(h(5)) || got.From.Location() != time.UTC || got.To.Location() != time.UTC {
			t.Fatalf("evento = %+v", got)
		}
	})

	t.Run("en curso: hasta until", func(t *testing.T) {
		got := ExclusionPeriod{StartedAt: h(1)}.KnownEvent(h(100))
		if !got.From.Equal(h(1)) || !got.To.Equal(h(100)) {
			t.Fatalf("evento = %+v", got)
		}
	})
}
