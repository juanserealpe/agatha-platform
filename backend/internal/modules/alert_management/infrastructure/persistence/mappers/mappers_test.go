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
	a := entities.Alert{
		ID: 123, Code: "ALR-000123", MeasuringPointID: 11556, EventType: entities.EventTypeLeak, State: entities.AlertActive,
		StartedAt: at, OpenedAt: at, CurrentLevel: entities.LevelNaranja, MaxLevel: entities.LevelRojo, LastEvaluatedAt: at,
		Runtime: entities.Runtime{PrevZone: entities.ZoneBand, Flips: []time.Time{flip}, LastFlipAt: &flip,
			Recent: []entities.HourValue{{At: at, I: 0.2}}, CountdownSentFor: entities.LevelAmarillo},
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
	if cols := ProcessingColumns(a); cols["classification"] != nil || cols["label"] != nil {
		t.Fatal("el procesamiento nunca escribe columnas de gestión")
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
