package services

import (
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
)

func TestValidateDateRange(t *testing.T) {
	v := NewDashboardValidator(testLogger())
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  error
	}{
		{name: "inicio antes del fin", start: base, end: base.Add(time.Hour), want: nil},
		{name: "inicio igual al fin", start: base, end: base, want: domain.ErrInvalidDateRange},
		{name: "inicio después del fin", start: base.Add(time.Hour), end: base, want: domain.ErrInvalidDateRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v.ValidateDateRange(tt.start, tt.end); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, se esperaba %v", err, tt.want)
			}
		})
	}
}

func TestValidateNotFuture(t *testing.T) {
	v := NewDashboardValidator(testLogger())

	if err := v.ValidateNotFuture(time.Now().Add(-time.Minute), domain.ErrStartDateFuture); err != nil {
		t.Fatalf("una fecha pasada no debe fallar: %v", err)
	}

	if err := v.ValidateNotFuture(time.Now().Add(time.Hour), domain.ErrEndDateFuture); !errors.Is(err, domain.ErrEndDateFuture) {
		t.Fatalf("una fecha futura debe devolver el error recibido, se obtuvo %v", err)
	}
}
