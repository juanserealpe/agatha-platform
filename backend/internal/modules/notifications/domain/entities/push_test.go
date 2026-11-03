package entities

import (
	"errors"
	"testing"
)

func TestPushValidate(t *testing.T) {
	cases := []struct {
		name string
		push Push
		ok   bool
	}{
		{"topic", Push{Topic: "alerts", Title: "t", Body: "b"}, true},
		{"token", Push{Token: "abc", Title: "t", Body: "b"}, true},
		{"sin destino", Push{Title: "t", Body: "b"}, false},
		{"topic y token a la vez", Push{Topic: "alerts", Token: "abc", Title: "t", Body: "b"}, false},
		{"sin título", Push{Topic: "alerts", Body: "b"}, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.push.Validate()
			if tt.ok && err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrInvalidPush) {
				t.Fatalf("err = %v, se esperaba ErrInvalidPush", err)
			}
		})
	}
}
