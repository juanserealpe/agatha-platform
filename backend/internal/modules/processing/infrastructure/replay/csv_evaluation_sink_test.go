package replay

import (
	"bytes"
	"context"
	"encoding/csv"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

func TestCSVEvaluationSink(t *testing.T) {
	first := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	sink := NewCSVEvaluationSink("fixed", first)
	ctx := context.Background()

	last, _ := sink.GetLastEvaluatedAt(ctx, 1)
	if !last.Equal(first.Add(-time.Hour)) {
		t.Fatalf("last inicial = %v, se esperaba %v", last, first.Add(-time.Hour))
	}

	i := 0.42
	version := 1
	validFrom := first.Add(-24 * time.Hour)
	rows := []entities.FeatureEvaluation{
		{MeasuringPointID: 1, WindowHours: 24, EvaluatedAt: first, Status: entities.StatusCalculated, I: &i,
			Flags: []string{"MAD_FLOOR_APPLIED_P", "CONSTANT_SERIES_B1"}, BaselineVersion: &version, BaselineValidFrom: &validFrom},
		{MeasuringPointID: 1, WindowHours: 6, EvaluatedAt: first.Add(time.Hour), Status: entities.StatusNoCalculable,
			NoCalcReason: entities.ReasonLowCoverage, Flags: []string{}},
	}

	if n, err := sink.SaveAll(ctx, rows); err != nil || n != 2 {
		t.Fatalf("SaveAll = %d, %v", n, err)
	}
	if last, _ := sink.GetLastEvaluatedAt(ctx, 1); !last.Equal(first) {
		t.Fatalf("last = %v: solo cuenta la ventana de 24 h", last)
	}

	var buf bytes.Buffer
	if err := sink.WriteCSV(&buf); err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || len(records[0]) != len(Header) {
		t.Fatalf("líneas = %d, columnas = %d", len(records), len(records[0]))
	}

	col := func(name string) int {
		for i, h := range Header {
			if h == name {
				return i
			}
		}
		t.Fatalf("columna %s inexistente", name)
		return -1
	}

	r1, r2 := records[1], records[2]
	checks := []struct{ got, want string }{
		{r1[col("i")], "0.42"},
		{r1[col("flags")], "MAD_FLOOR_APPLIED_P|CONSTANT_SERIES_B1"},
		{r1[col("mode")], "fixed"},
		{r1[col("baseline_valid_from")], "2026-07-07T00:00:00Z"},
		{r1[col("evaluated_at")], "2026-07-08T00:00:00Z"},
		{r2[col("i")], ""},
		{r2[col("no_calc_reason")], "LOW_COVERAGE"},
		{r2[col("baseline_version")], ""},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("celda = %q, se esperaba %q", c.got, c.want)
		}
	}
}
