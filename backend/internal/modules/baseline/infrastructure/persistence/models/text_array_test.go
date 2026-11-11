package models

import (
	"reflect"
	"testing"
)

func TestTextArray(t *testing.T) {
	v, err := TextArray{"A", "B"}.Value()
	if err != nil || v != "{A,B}" {
		t.Fatalf("Value = %v (err %v)", v, err)
	}
	if v, _ := (TextArray{}).Value(); v != "{}" {
		t.Fatalf("Value vacío = %v", v)
	}

	tests := []struct {
		name string
		src  any
		want TextArray
	}{
		{"nil", nil, TextArray{}},
		{"vacío", "{}", TextArray{}},
		{"string", "{BASELINE_REVIEW,NO_EXCLUSION_HISTORY}", TextArray{"BASELINE_REVIEW", "NO_EXCLUSION_HISTORY"}},
		{"bytes con comillas", []byte(`{"A B",C}`), TextArray{"A B", "C"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TextArray
			if err := got.Scan(tt.src); err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Scan = %v (err %v), se esperaba %v", got, err, tt.want)
			}
		})
	}

	var bad TextArray
	if err := bad.Scan(42); err == nil {
		t.Fatal("un tipo no soportado debe fallar")
	}
}
