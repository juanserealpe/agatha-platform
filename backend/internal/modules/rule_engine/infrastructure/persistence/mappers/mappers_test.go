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
