package mappers

import (
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

func TestRuleEvaluation_RoundTripKeepsReasons(t *testing.T) {
	row := entities.RuleEvaluation{
		ID: 7, MeasuringPointID: 11556, EventType: entities.EventTypeLeak,
		EvaluatedAt:    time.Date(2026, 10, 15, 17, 0, 0, 0, time.UTC),
		IndicatorValue: 0.22, RawLevel: entities.LevelAmarillo, ComputedLevel: entities.LevelNaranja,
		Level: entities.LevelNaranja, Rule: entities.RulePersistence, InEpisode: true, HoursAbove: 14, HoursInLevel: 3,
		ThresholdVersionID: 3, EpisodeEvent: entities.EpisodeEventNone,
		Reasons: []entities.Reason{
			{Code: entities.ReasonLeakThreshold, Level: entities.LevelAmarillo, I: 0.22, Threshold: 0.18},
			{Code: entities.ReasonLeakPersistence, Level: entities.LevelNaranja, Hours: 12, From: entities.LevelAmarillo},
		},
		CreatedAt: time.Date(2026, 10, 15, 17, 35, 0, 0, time.UTC),
	}

	m := ToRuleEvaluationModel(row)
	want := `[{"code":"LEAK_THRESHOLD","level":"AMARILLO","data":{"i":0.22,"threshold":0.18}},` +
		`{"code":"LEAK_PERSISTENCE","level":"NARANJA","data":{"hours":12,"from":"AMARILLO"}}]`
	if m.Reasons != want {
		t.Fatalf("reasons = %s\nse esperaba %s", m.Reasons, want)
	}
	if got := ToRuleEvaluation(m); !reflect.DeepEqual(got, row) {
		t.Fatalf("ida y vuelta = %+v\nse esperaba  %+v", got, row)
	}
}

func TestRuleEvaluation_NoReasonsIsAnEmptyArray(t *testing.T) {
	m := ToRuleEvaluationModel(entities.RuleEvaluation{RawLevel: entities.LevelNormal})
	if m.Reasons != "[]" {
		t.Fatalf("reasons = %q, se esperaba []", m.Reasons)
	}
	if got := ToRuleEvaluation(m); got.Reasons != nil {
		t.Fatalf("reasons = %+v, se esperaba nil", got.Reasons)
	}
}

func TestRuleEvaluation_RoundTripKeepsV3Fields(t *testing.T) {
	dPl := 8.0
	row := entities.RuleEvaluation{
		RawLevel: entities.LevelNaranja, ComputedLevel: entities.LevelAmarillo, Level: entities.LevelAmarillo,
		RunAmarillo: 18, RunNaranja: 1, BelowRun: 0, DPl: &dPl,
		Reasons: []entities.Reason{
			{Code: entities.ReasonNaranjaNotConfirmed, Level: entities.LevelAmarillo, From: entities.LevelNaranja, DPl: &dPl, DPlMin: 12},
			{Code: entities.ReasonLeakPersistence, Level: entities.LevelNaranja, Hours: 18, From: entities.LevelAmarillo, WindowHours: 24},
		},
		EvaluatedAt: time.Date(2026, 9, 19, 5, 0, 0, 0, time.UTC),
		CreatedAt:   time.Date(2026, 9, 19, 5, 35, 0, 0, time.UTC),
	}
	if got := ToRuleEvaluation(ToRuleEvaluationModel(row)); !reflect.DeepEqual(got, row) {
		t.Fatalf("ida y vuelta = %+v\nse esperaba  %+v", got, row)
	}
}

func TestThresholdVersion_RoundTripKeepsV2Columns(t *testing.T) {
	p := entities.DefaultThresholdParams()
	min, window, minHours := 12.0, 24, 18
	p.ConfirmNaranjaWithPlanitud, p.ConfirmRojoWithPlanitud, p.DPlMin = true, true, &min
	p.PersistenceMode, p.PersistenceWindowHours, p.PersistenceMinHours = entities.PersistenceWindow, &window, &minHours
	v := entities.ThresholdVersion{
		ID: 2, MeasuringPointID: 11556, EventType: entities.EventTypeLeak, Version: 2, ThresholdParams: p,
		ValidFrom: time.Date(2026, 10, 24, 2, 40, 0, 0, time.UTC), Reason: "r", CreatedBy: "c",
		CreatedAt: time.Date(2026, 10, 24, 2, 40, 0, 0, time.UTC),
	}
	if got := ToThresholdVersion(ToThresholdVersionModel(v)); !reflect.DeepEqual(got, v) {
		t.Fatalf("ida y vuelta = %+v\nse esperaba  %+v", got, v)
	}

	// Una fila previa a la columna (modo vacío) se lee como STRICT.
	legacy := ToThresholdVersionModel(entities.ThresholdVersion{ThresholdParams: entities.DefaultThresholdParams()})
	legacy.PersistenceMode = ""
	if got := ToThresholdVersion(legacy); got.PersistenceMode != entities.PersistenceStrict {
		t.Fatalf("modo = %q, se esperaba STRICT", got.PersistenceMode)
	}
}
