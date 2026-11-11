package entities

import "testing"

func keysOf(defs []SeriesDefinition) []string {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = d.Key
	}
	return keys
}

func TestSeriesCatalog_EveryDefinitionIsConsistentWithItsSource(t *testing.T) {
	seen := map[string]bool{}

	for _, d := range SeriesCatalog() {
		if d.Key == "" || d.Name == "" || d.Unit == "" {
			t.Errorf("definición incompleta: %+v", d)
		}
		if seen[d.Key] {
			t.Errorf("key repetida en el catálogo: %q", d.Key)
		}
		seen[d.Key] = true

		switch d.Source {
		case SourceMeasurement:
			if d.MetricID == 0 || d.Column != "" {
				t.Errorf("%q: una serie de medición necesita MetricID y no Column: %+v", d.Key, d)
			}
		case SourceEvaluation:
			if d.Column == "" || (d.WindowHours != 6 && d.WindowHours != 24) || d.MetricID != 0 {
				t.Errorf("%q: una serie de evaluación necesita Column y ventana 6|24: %+v", d.Key, d)
			}
		default:
			t.Errorf("%q: fuente desconocida %q", d.Key, d.Source)
		}
	}
}

func TestSeriesCatalog_ReturnsACopy(t *testing.T) {
	first := SeriesCatalog()
	first[0].Key = "modificada"

	if SeriesCatalog()[0].Key == "modificada" {
		t.Fatal("modificar el resultado alteró el catálogo interno")
	}
}

func TestResolveSeries(t *testing.T) {
	all := keysOf(SeriesCatalog())

	tests := []struct {
		name        string
		keys        []string
		wantKeys    []string
		wantUnknown string
		wantOK      bool
	}{
		{name: "sin keys devuelve el catálogo completo", keys: nil, wantKeys: all, wantOK: true},
		{name: "una key", keys: []string{"b2"}, wantKeys: []string{"b2"}, wantOK: true},
		{
			name:     "respeta el orden del catálogo, no el del pedido",
			keys:     []string{"indicador", "b1"},
			wantKeys: []string{"b1", "indicador"},
			wantOK:   true,
		},
		{name: "keys repetidas se devuelven una vez", keys: []string{"b1", "b1"}, wantKeys: []string{"b1"}, wantOK: true},
		{name: "key desconocida", keys: []string{"b1", "presion"}, wantUnknown: "presion", wantOK: false},
		{name: "es sensible a mayúsculas", keys: []string{"B1"}, wantUnknown: "B1", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, unknown, ok := ResolveSeries(tt.keys)

			if ok != tt.wantOK || unknown != tt.wantUnknown {
				t.Fatalf("ok=%v unknown=%q, se esperaba ok=%v unknown=%q", ok, unknown, tt.wantOK, tt.wantUnknown)
			}
			if !tt.wantOK {
				if defs != nil {
					t.Fatalf("con una key desconocida no se devuelven definiciones: %v", keysOf(defs))
				}
				return
			}

			got := keysOf(defs)
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("keys = %v, se esperaba %v", got, tt.wantKeys)
			}
			for i := range got {
				if got[i] != tt.wantKeys[i] {
					t.Fatalf("keys = %v, se esperaba %v", got, tt.wantKeys)
				}
			}
		})
	}
}
