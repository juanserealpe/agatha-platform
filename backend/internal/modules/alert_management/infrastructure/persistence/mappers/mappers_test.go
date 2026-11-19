package mappers

import (
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

func TestAlert_RoundTripKeepsManagementAndRuntime(t *testing.T) {
	at := time.Date(2026, 8, 24, 16, 0, 0, 0, time.UTC)
	flip := at.Add(-time.Hour)
	dPl := 4.6
	a := entities.Alert{
		ID: 123, Code: "ALR-000123", MeasuringPointID: 11556, EventType: entities.EventTypeLeak, Events: []entities.EventType{entities.EventTypeLeak, entities.EventTypeMovement},
		MotionState: "DESPLAZADO", TempState: "TEMP_ESTABLE", TempStableFrom: &at, LastMotionEvaluatedAt: &at, State: entities.AlertActive,
		StartedAt: at, OpenedAt: at, CurrentLevel: entities.LevelNaranja, MaxLevel: entities.LevelRojo, LastEvaluatedAt: at,
		OpeningIndicatorValue: 0.18, LastIndicatorValue: 0.2, MaxIndicatorValue: 0.41,
		Runtime: entities.Runtime{PrevZone: entities.ZoneBand, Flips: []time.Time{flip}, LastFlipAt: &flip,
			Recent: []entities.HourValue{{At: at, I: 0.2}}, CountdownSentFor: entities.LevelAmarillo, Limited: true, NoLeakSent: true},
		LeakSignature: &entities.LeakSignature{DPl: &dPl, DPlMin: 10, Confirmed: false},
		Management: entities.Management{Status: entities.StatusClassifiedAndLabel, Classification: entities.ClassificationFalseAlarm,
			ClassifiedAt: &at, ClassifiedOrigin: entities.OriginMobile, ClassifiedDevice: "Celular 1", ClassifiedRecordedAt: &at,
			ClassifiedClientRecordID: "6f1c0e3a-1b2c-4d5e-8f90-1234567890ab",
			Label:                    entities.LabelOther, LabelOtherText: "Prueba", LabeledAt: &at, LabeledOrigin: entities.OriginWeb, LabeledRecordedAt: &at},
		Source: entities.SourceLive, CreatedAt: at,
	}
	m := ToAlertModel(a)
	m.Code = "ALR-000123"
	got := ToAlert(m)
	if !reflect.DeepEqual(got, a) {
		t.Fatalf("ida y vuelta:\n got %+v\nwant %+v", got, a)
	}
	if m.ManagementStatus != string(entities.StatusClassifiedAndLabel) {
		t.Fatalf("estado = %s", m.ManagementStatus)
	}
	cols := ProcessingColumns(a)
	if cols["classification"] != nil || cols["label"] != nil {
		t.Fatal("el procesamiento nunca escribe columnas de gestión")
	}
	if *(cols["leak_d_pl_min"].(*float64)) != 10 || *(cols["leak_confirmed"].(*bool)) || *(cols["leak_d_pl"].(*float64)) != 4.6 {
		t.Fatalf("el procesamiento debe escribir la firma de fuga: %v", cols)
	}
	if cols["max_indicator_value"] != 0.41 {
		t.Fatalf("el procesamiento debe escribir el máximo del índice: %v", cols["max_indicator_value"])
	}
}

func TestToLevelNotice_RebuildsExitEscalationAndPersistence(t *testing.T) {
	twelve, six, eighteen := 12, 6, 18
	row := models.RuleEvaluationReadModel{
		ID: 7, MeasuringPointID: 11556, EventType: "FUGA", EvaluatedAt: time.Date(2026, 8, 24, 22, 0, 0, 0, time.UTC),
		IndicatorValue: 0.22, RawLevel: "AMARILLO", ComputedLevel: "NARANJA", Level: "NARANJA", InEpisode: true,
		EpisodeEvent: "NONE", RunAmarillo: 12, Reasons: `[{"code":"LEAK_THRESHOLD","level":"AMARILLO","data":{"i":0.22,"threshold":0.16}},{"code":"LEAK_PERSISTENCE","level":"NARANJA","data":{"hours":12,"from":"AMARILLO"}}]`,
		Amarillo: 0.16, Naranja: 0.25, Rojo: 0.35, ExitMargin: 0.02,
		EscalateAmarilloAfterHours: &twelve, EscalateNaranjaAfterHours: &six, PersistenceMode: "STRICT",
	}
	n := ToLevelNotice(row)
	if n.Exit.Amarillo != 0.14 || n.Exit.Naranja != 0.23 || *n.EscalationAmarillo != 12 || n.PersistenceHours != 12 || !n.Calculated() {
		t.Fatalf("notice = %+v", n)
	}

	row.PersistenceMode, row.PersistenceMinHours, row.EscalateNaranjaAfterHours = "WINDOW", &eighteen, nil
	n = ToLevelNotice(row)
	if *n.EscalationAmarillo != 18 || n.EscalationNaranja != nil {
		t.Fatalf("WINDOW: escalamientos = %v / %v", n.EscalationAmarillo, n.EscalationNaranja)
	}
}

// Sin firma de fuga (versión sin d_pl_min) las tres columnas quedan NULL y la
// ida y vuelta devuelve nil; una firma sin d_pl conserva el nulo.
func TestAlert_LeakSignatureNullsAndMissingDPl(t *testing.T) {
	m := ToAlertModel(entities.Alert{})
	if m.LeakDPl != nil || m.LeakDPlMin != nil || m.LeakConfirmed != nil {
		t.Fatalf("columnas = %+v", m)
	}
	if got := ToAlert(m).LeakSignature; got != nil {
		t.Fatalf("firma = %+v", got)
	}
	cols := ProcessingColumns(entities.Alert{})
	for _, k := range []string{"leak_d_pl", "leak_d_pl_min", "leak_confirmed"} {
		if _, ok := cols[k]; !ok {
			t.Errorf("falta %s: el procesamiento debe poder volver a NULL", k)
		}
	}
	got := ToAlert(ToAlertModel(entities.Alert{LeakSignature: &entities.LeakSignature{DPlMin: 10}})).LeakSignature
	if got == nil || got.DPl != nil || got.DPlMin != 10 || got.Confirmed {
		t.Fatalf("firma sin d_pl = %+v", got)
	}
}

func TestToLevelNotice_CarriesPlanitudAndTheLimitedLevel(t *testing.T) {
	dPl, dPlMin := 4.6, 10.0
	row := models.RuleEvaluationReadModel{
		ID: 7, MeasuringPointID: 11556, EventType: "FUGA", EvaluatedAt: time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC),
		IndicatorValue: 0.36, RawLevel: "ROJO", ComputedLevel: "AMARILLO", Level: "AMARILLO", InEpisode: true, EpisodeEvent: "NONE",
		Reasons:  `[{"code":"LEAK_THRESHOLD","level":"ROJO","data":{"i":0.36,"threshold":0.35}},{"code":"NARANJA_NOT_CONFIRMED","level":"AMARILLO","data":{"from":"ROJO","d_pl":4.6,"d_pl_min":10}}]`,
		Amarillo: 0.16, Naranja: 0.25, Rojo: 0.35, DPl: &dPl, DPlMin: &dPlMin,
	}
	n := ToLevelNotice(row)
	if n.DPl == nil || *n.DPl != 4.6 || n.DPlMin == nil || *n.DPlMin != 10 || n.LimitedFrom != entities.LevelRojo || !n.Limited() {
		t.Fatalf("notice = %+v", n)
	}
	row.Reasons, row.DPl, row.DPlMin = `[]`, nil, nil
	n = ToLevelNotice(row)
	if n.DPl != nil || n.DPlMin != nil || n.Limited() {
		t.Fatalf("sin planitud: notice = %+v", n)
	}
}
