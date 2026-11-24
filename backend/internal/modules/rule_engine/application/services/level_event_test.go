package services_test

import (
	"encoding/json"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// v5 retiró d_pl y d_pl_min del evento (la confirmación por planitud ya no existe).
func TestLevelEvaluatedFromRow_DoesNotCarryPlanitudFields(t *testing.T) {
	row := entities.RuleEvaluation{EventType: entities.EventTypeLeak, RawLevel: entities.LevelNormal,
		ComputedLevel: entities.LevelNormal, Level: entities.LevelNormal}
	ev := services.LevelEvaluatedFromRow(row, entities.DefaultThresholdParams(), now)
	if ev.SchemaVersion != 5 || contract.SchemaVersion != 5 {
		t.Fatalf("schema_version = %d; se esperaba 5", ev.SchemaVersion)
	}

	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"d_pl", "d_pl_min"} {
		if _, present := fields[name]; present {
			t.Fatalf("%s ya no forma parte del contrato: %s", name, raw)
		}
	}
}
