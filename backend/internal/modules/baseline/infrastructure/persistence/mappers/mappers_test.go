package mappers

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

var cot = time.FixedZone("COT", -5*3600)

func sampleVersion() entities.BaselineVersion {
	p90, minElevated := 6.2, 55.0
	validFrom := time.Date(2026, 10, 10, 8, 17, 0, 0, cot)

	return entities.BaselineVersion{
		MeasuringPointID: 11556,
		ValidFrom:        validFrom,
		HistoryFrom:      validFrom.Add(-90 * 24 * time.Hour),
		HistoryTo:        validFrom,
		Signal: entities.SignalBaseline{
			MedianB2: 53, ThetaD: 67, P90Planitud: &p90, MinElevatedB2: &minElevated,
			DeltaMin: 1, NRecords: 6361, NPlanitudRecords: 6000,
		},
		Components: []entities.ComponentStat{
			{WindowHours: 6, Component: features.ComponentD, Median: 1, MAD: 0.1, NWindows: 400},
			{WindowHours: 24, Component: features.ComponentC, Median: 0.6, MAD: 0.05, NWindows: 246},
		},
		Confidence:       entities.ConfidenceNormal,
		ExcludedPeriods:  []entities.ExcludedPeriod{{From: validFrom.Add(-48 * time.Hour), To: validFrom.Add(-24 * time.Hour)}},
		Estimation:       entities.EstimationSummary{Passes: 5, Converged: false, ExcludedRatio: 0.2176, CapReached: true},
		Flags:            []entities.Flag{entities.FlagExclusionCapReached, entities.FlagBaselineReview},
		Trigger:          entities.TriggerManual,
		AlgorithmVersion: features.AlgorithmVersion,
		CreatedAt:        validFrom,
	}
}

func TestToBaselineVersionModel_StoresEverythingInUTC(t *testing.T) {
	v := sampleVersion()

	m, err := ToBaselineVersionModel(v)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	for name, ts := range map[string]time.Time{"valid_from": m.ValidFrom, "history_from": m.HistoryFrom, "history_to": m.HistoryTo, "created_at": m.CreatedAt} {
		if ts.Location() != time.UTC {
			t.Errorf("%s no está en UTC: %v", name, ts)
		}
	}
	if m.MedianB2 != 53 || m.ThetaD != 67 || *m.P90Planitud != 6.2 || m.NSignalRecords != 6361 || m.Confidence != "NORMAL" ||
		m.ExclusionPasses != 5 || m.ExcludedRatio != 0.2176 || m.Trigger != "MANUAL" || len(m.ComponentStats) != 2 {
		t.Fatalf("modelo mal armado: %+v", m)
	}
	if !reflect.DeepEqual([]string(m.Flags), []string{"EXCLUSION_CAP_REACHED", "BASELINE_REVIEW"}) {
		t.Fatalf("flags = %v", m.Flags)
	}

	// excluded_periods guarda solo fechas: [{from, to}].
	var periods []map[string]any
	if err := json.Unmarshal([]byte(m.ExcludedPeriods), &periods); err != nil || len(periods) != 1 || len(periods[0]) != 2 {
		t.Fatalf("excluded_periods = %s (err %v)", m.ExcludedPeriods, err)
	}
}

func TestBaselineVersion_RoundTrip(t *testing.T) {
	v := sampleVersion()
	m, err := ToBaselineVersionModel(v)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	m.ID, m.Version = 7, 3

	got, err := ToBaselineVersion(m)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if got.ID != 7 || got.Version != 3 || got.ValidTo != nil || !got.ValidFrom.Equal(v.ValidFrom) || got.ValidFrom.Location() != time.UTC {
		t.Fatalf("cabecera mal reconstruida: %+v", got)
	}
	if !reflect.DeepEqual(got.Signal, v.Signal) || !reflect.DeepEqual(got.Components, v.Components) || !reflect.DeepEqual(got.Flags, v.Flags) {
		t.Fatalf("señal, componentes o flags distintos:\n%+v\n%+v", got, v)
	}
	if len(got.ExcludedPeriods) != 1 || !got.ExcludedPeriods[0].From.Equal(v.ExcludedPeriods[0].From) {
		t.Fatalf("periodos = %v", got.ExcludedPeriods)
	}
	if got.Estimation != v.Estimation {
		t.Fatalf("estimación = %+v, se esperaba %+v", got.Estimation, v.Estimation)
	}
}

func TestToBaselineVersion_CapReachedComesFromTheFlag(t *testing.T) {
	got, err := ToBaselineVersion(models.BaselineVersionModel{Flags: models.TextArray{"BASELINE_REVIEW"}})
	if err != nil || got.Estimation.CapReached {
		t.Fatalf("sin EXCLUSION_CAP_REACHED no hay tope: %+v (err %v)", got.Estimation, err)
	}
}

func TestToBaselineVersion_SortsComponentStats(t *testing.T) {
	m := models.BaselineVersionModel{ComponentStats: []models.BaselineComponentStatModel{
		{WindowHours: 24, Component: "C"}, {WindowHours: 24, Component: "D"},
		{WindowHours: 6, Component: "EX"}, {WindowHours: 6, Component: "D"}, {WindowHours: 6, Component: "P"},
	}}

	got, err := ToBaselineVersion(m)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	order := []string{}
	for _, st := range got.Components {
		order = append(order, st.Component+"@"+map[int]string{6: "6", 24: "24"}[st.WindowHours])
	}
	if want := []string{"D@6", "P@6", "EX@6", "D@24", "C@24"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("orden = %v, se esperaba %v", order, want)
	}
}

func TestToBaselineVersion_ValidToAndLegacyPeriods(t *testing.T) {
	validTo := time.Date(2026, 10, 17, 3, 0, 0, 0, cot)
	m := models.BaselineVersionModel{
		ValidTo: &validTo,
		// filas viejas guardaban peak_i; al leer se ignora.
		ExcludedPeriods: `[{"from":"2026-07-23T00:00:00Z","to":"2026-08-02T00:00:00Z","peak_i":0.99}]`,
	}

	got, err := ToBaselineVersion(m)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.ValidTo == nil || got.ValidTo.Location() != time.UTC || !got.ValidTo.Equal(validTo) {
		t.Fatalf("valid_to = %v", got.ValidTo)
	}
	if len(got.ExcludedPeriods) != 1 || got.ExcludedPeriods[0].To.Day() != 2 {
		t.Fatalf("periodos = %v", got.ExcludedPeriods)
	}
}

func TestToBaselineVersion_EmptyAndInvalidPeriods(t *testing.T) {
	got, err := ToBaselineVersion(models.BaselineVersionModel{})
	if err != nil || got.ExcludedPeriods == nil || len(got.ExcludedPeriods) != 0 {
		t.Fatalf("sin periodos se espera lista vacía: %v (err %v)", got.ExcludedPeriods, err)
	}

	if _, err := ToBaselineVersion(models.BaselineVersionModel{ExcludedPeriods: "{no json"}); err == nil {
		t.Fatal("un JSON inválido en excluded_periods debe fallar")
	}
}

func TestToSamples_PivotsRowsByTimestamp(t *testing.T) {
	ids := MetricIDs{B1: 12, B2: 13, Planitud: 17}
	ts1 := time.Date(2026, 10, 1, 5, 0, 0, 0, cot)
	ts2 := ts1.Add(15 * time.Minute)

	got := ToSamples([]models.RawSampleRowModel{
		{Timestamp: ts1, MetricID: 12, Value: 40},
		{Timestamp: ts1, MetricID: 13, Value: 50},
		{Timestamp: ts1, MetricID: 17, Value: 5},
		{Timestamp: ts2, MetricID: 13, Value: math.NaN()},
		{Timestamp: ts2, MetricID: 99, Value: 1}, // métrica ajena: se ignora
	}, ids)

	if len(got) != 2 {
		t.Fatalf("se esperaban 2 muestras, hay %d", len(got))
	}
	first := got[0]
	if !first.T.Equal(ts1) || first.T.Location() != time.UTC ||
		!first.HasB1 || first.B1 != 40 || !first.HasB2 || first.B2 != 50 || !first.HasPl || first.Pl != 5 {
		t.Fatalf("primera muestra mal armada: %+v", first)
	}
	if got[1].HasB2 || got[1].HasB1 || got[1].HasPl {
		t.Fatalf("un NaN cuenta como ausente: %+v", got[1])
	}

	if empty := ToSamples(nil, ids); empty == nil || len(empty) != 0 {
		t.Fatalf("sin filas se espera lista vacía: %v", empty)
	}
}
