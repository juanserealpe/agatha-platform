package mappers

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

var ids = MetricIDs{B1: 12, B2: 13, Planitud: 18}

func TestConfigVersion(t *testing.T) {
	if got := ids.ConfigVersion(); got != "m=12,13,18" {
		t.Fatalf("config_version = %s", got)
	}
}

func TestToSamplesPivotsByTimestamp(t *testing.T) {
	t0 := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(15 * time.Minute)

	rows := []models.RawSampleRowModel{
		{Timestamp: t0, MetricID: 12, Value: 1},
		{Timestamp: t0, MetricID: 13, Value: 2},
		{Timestamp: t0, MetricID: 18, Value: 0.3},
		{Timestamp: t1, MetricID: 13, Value: math.NaN()},
		{Timestamp: t1, MetricID: 18, Value: 0.4},
	}

	got := ToSamples(rows, ids)

	if len(got) != 2 {
		t.Fatalf("samples = %d, se esperaban 2", len(got))
	}
	if s := got[0]; !s.HasB1 || !s.HasB2 || !s.HasPl || s.B1 != 1 || s.B2 != 2 || s.Pl != 0.3 {
		t.Fatalf("sample 0 = %+v", s)
	}
	if s := got[1]; s.HasB1 || s.HasB2 || !s.HasPl {
		t.Fatalf("sample 1 = %+v: NaN y ausente deben quedar Has* = false", s)
	}
}

func TestFeatureEvaluationRoundTrip(t *testing.T) {
	at := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	f := func(v float64) *float64 { return &v }
	id, ver, seq := int64(7), 3, int64(99)
	maxTs := at.Add(-15 * time.Minute)

	calculated := entities.FeatureEvaluation{
		MeasuringPointID: 11556, WindowHours: 24, EvaluatedAt: at, WindowStart: at.Add(-24 * time.Hour),
		Status: entities.StatusCalculated, NExpected: 96, NReceived: 95, NValid: 94, Coverage: 97.9,
		D: f(1), PHours: f(2), Ex: f(3), C: f(0.4), DPl: f(5),
		ZD: f(1), ZP: f(2), ZEx: f(3), ZC: f(4), SD: f(0.1), SP: f(0.2), SEx: f(0.3), SC: f(0.4), I: f(0.5),
		LowConfidence: true, Silent: true, Flags: []string{"MAD_FLOOR_APPLIED_P"},
		BaselineVersionID: &id, BaselineVersion: &ver,
		AlgorithmVersion: "1.0.0", ConfigVersion: "m=12,13,18", Trigger: entities.TriggerEvent,
		SourceBatchSeq: &seq, InputMaxTs: &maxTs, CreatedAt: at.Add(time.Minute),
	}

	noCalc := entities.FeatureEvaluation{
		MeasuringPointID: 11556, WindowHours: 6, EvaluatedAt: at, WindowStart: at.Add(-6 * time.Hour),
		Status: entities.StatusNoCalculable, NoCalcReason: entities.ReasonLowCoverage, NExpected: 24,
		CFrom24h: true, Flags: []string{}, AlgorithmVersion: "1.0.0", ConfigVersion: "m=12,13,18",
		Trigger: entities.TriggerReconciler, CreatedAt: at,
	}

	for _, e := range []entities.FeatureEvaluation{calculated, noCalc} {
		m := ToFeatureEvaluationModel(e)
		if (e.NoCalcReason == entities.ReasonNone) != (m.NoCalcReason == nil) {
			t.Fatalf("no_calc_reason = %v para %s", m.NoCalcReason, e.NoCalcReason)
		}
		if got := ToFeatureEvaluation(m); !reflect.DeepEqual(got, e) {
			t.Fatalf("ida y vuelta:\n got  %+v\n want %+v", got, e)
		}
	}
}
