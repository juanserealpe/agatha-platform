package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

func TestNewPushMessage_MapsTheContract(t *testing.T) {
	started := time.Date(2026, 10, 18, 19, 32, 0, 0, time.UTC)
	n := entities.AlertNotice{
		MessageID: 9, AlertCode: "ALR-000142", Code: "UP_NARANJA_FAST", Level: entities.LevelNaranja, EventType: entities.EventLeak,
		StartedAt: started, Classification: entities.ClassificationConfirmed,
	}
	got, err := entities.NewPushMessage(n, "Sensor PK34-470")
	if err != nil {
		t.Fatal(err)
	}
	want := entities.PushMessage{
		MessageID: 9, AlertID: "ALR-000142", Level: entities.PushLevelOrange, Kind: entities.PushKindLevelUp,
		SensorID: "Sensor PK34-470", PK: "34+470", Event: entities.PushEventLeak, StartedAt: started,
		Classification: entities.PushClassificationConfirmed,
	}
	if got != want {
		t.Fatalf("mensaje = %+v, se esperaba %+v", got, want)
	}
}

// Sin dispositivo, evento conocido ni clasificación, los opcionales quedan vacíos.
func TestNewPushMessage_OptionalFieldsAreEmpty(t *testing.T) {
	got, err := entities.NewPushMessage(entities.AlertNotice{AlertCode: "ALR-000007", Code: "OPEN", Level: entities.LevelAmarillo, EventType: "OTRO"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != entities.PushKindNew || got.Level != entities.PushLevelYellow ||
		got.SensorID != "" || got.PK != "" || got.Event != "" || got.Classification != "" {
		t.Fatalf("mensaje = %+v", got)
	}
}

func TestNewPushMessage_RejectsWhatThePolicyFilters(t *testing.T) {
	for _, n := range []entities.AlertNotice{
		{AlertCode: "ALR-000001", Code: "OPEN", Level: "NORMAL"},
		{AlertCode: "ALR-000001", Code: "DOWNGRADE", Level: entities.LevelAmarillo},
		{Code: "OPEN", Level: entities.LevelRojo},
	} {
		if _, err := entities.NewPushMessage(n, ""); !errors.Is(err, entities.ErrNotPushable) {
			t.Fatalf("NewPushMessage(%+v) error = %v", n, err)
		}
	}
}

func TestPKFromDeviceName(t *testing.T) {
	tests := map[string]string{
		"Sensor PK34-470":  "34+470",
		"pk 34+470":        "34+470",
		"PK 034 + 470 sur": "34+470",
		"PK12-350":         "12+350",
		"PK34-4701":        "", // los metros son exactamente 3 dígitos
		"Sensor 7":         "",
		"":                 "",
	}
	for name, want := range tests {
		if got := entities.PKFromDeviceName(name); got != want {
			t.Errorf("PKFromDeviceName(%q) = %q, se esperaba %q", name, got, want)
		}
	}
}
