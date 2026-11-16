package services_test

import (
	"encoding/json"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// d_pl y d_pl_min viajan en TODA hora calculada, no solo dentro del motivo
// NARANJA_NOT_CONFIRMED.
func TestLevelEvaluatedFromRow_CarriesDPlAndDPlMinOnEveryHour(t *testing.T) {
	p := entities.DefaultThresholdParams()
	p.ConfirmNaranjaWithPlanitud, p.ConfirmRojoWithPlanitud, p.DPlMin = true, true, f64(10)

	// Hora NORMAL, sin ningún motivo: igual lleva ambos.
	row := entities.RuleEvaluation{EventType: entities.EventTypeLeak, RawLevel: entities.LevelNormal,
		ComputedLevel: entities.LevelNormal, Level: entities.LevelNormal, DPl: f64(4.6)}
	ev := services.LevelEvaluatedFromRow(row, p, now)
	if ev.DPl == nil || *ev.DPl != 4.6 || ev.DPlMin == nil || *ev.DPlMin != 10 {
		t.Fatalf("d_pl = %v, d_pl_min = %v; se esperaba 4.6 y 10", ev.DPl, ev.DPlMin)
	}
	if len(ev.Reasons) != 0 {
		t.Fatalf("la hora NORMAL no tiene motivos: %+v", ev.Reasons)
	}

	// Es una copia: mutar el evento no toca la versión ni la fila.
	*ev.DPlMin = 99
	*ev.DPl = 99
	if *p.DPlMin != 10 || *row.DPl != 4.6 {
		t.Fatal("el evento no debe compartir los punteros con la versión ni con la fila")
	}
}

func TestLevelEvaluatedFromRow_UnknownDPlAndDPlMinAreNullAndOmitted(t *testing.T) {
	row := entities.RuleEvaluation{EventType: entities.EventTypeLeak, RawLevel: entities.LevelNormal,
		ComputedLevel: entities.LevelNormal, Level: entities.LevelNormal}
	ev := services.LevelEvaluatedFromRow(row, entities.DefaultThresholdParams(), now)
	if ev.DPl != nil || ev.DPlMin != nil {
		t.Fatalf("d_pl = %v, d_pl_min = %v; sin dato ambos son nil", ev.DPl, ev.DPlMin)
	}

	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["d_pl"]) != "null" {
		t.Fatalf("d_pl = %s; el campo existente sigue siendo null sin dato", fields["d_pl"])
	}
	if _, present := fields["d_pl_min"]; present {
		t.Fatalf("d_pl_min sin dato debe omitirse: %s", raw)
	}
}
