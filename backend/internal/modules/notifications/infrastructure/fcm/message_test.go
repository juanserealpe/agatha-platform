package fcm

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

func TestNewSendRequest_FollowsTheContract(t *testing.T) {
	m := entities.PushMessage{
		MessageID: 9, AlertID: "ALR-000142", Level: entities.PushLevelOrange, Kind: entities.PushKindLevelUp,
		SensorID: "Sensor PK34-470", PK: "34+470", Event: entities.PushEventLeak,
		StartedAt:      time.Date(2026, 10, 18, 19, 32, 0, 0, time.UTC),
		Classification: entities.PushClassificationConfirmed,
	}
	raw, err := json.Marshal(newSendRequest("token-fcm", m, false))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	want := map[string]any{
		"message": map[string]any{
			"token":   "token-fcm",
			"android": map[string]any{"priority": "HIGH"},
			"data": map[string]any{
				"messageId": "9", "alertId": "ALR-000142", "level": "ORANGE", "kind": "LEVEL_UP",
				"sensorId": "Sensor PK34-470", "pk": "34+470", "event": "LEAK",
				"startedAt": "2026-10-18T14:32:00-05:00", "classification": "CONFIRMED",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cuerpo:\n got %s\nwant %v", raw, want)
	}
}

// Sin bloque notification, sin opcionales vacíos y sin validate_only si no se pidió.
func TestNewSendRequest_OmitsEmptyOptionals(t *testing.T) {
	m := entities.PushMessage{AlertID: "ALR-000007", Level: entities.PushLevelYellow, Kind: entities.PushKindNew}
	raw, _ := json.Marshal(newSendRequest("t", m, false))
	var got struct {
		ValidateOnly *bool `json:"validate_only"`
		Message      struct {
			Notification any               `json:"notification"`
			Data         map[string]string `json:"data"`
		} `json:"message"`
	}
	_ = json.Unmarshal(raw, &got)
	if got.ValidateOnly != nil || got.Message.Notification != nil {
		t.Fatalf("cuerpo = %s", raw)
	}
	want := map[string]string{"alertId": "ALR-000007", "level": "YELLOW", "kind": "NEW"}
	if !reflect.DeepEqual(got.Message.Data, want) {
		t.Fatalf("data = %v, se esperaba %v", got.Message.Data, want)
	}
}

func TestNewSendRequest_ValidateOnly(t *testing.T) {
	raw, _ := json.Marshal(newSendRequest("t", entities.PushMessage{AlertID: "A", Level: "RED", Kind: "NEW"}, true))
	var got struct {
		ValidateOnly bool `json:"validate_only"`
	}
	_ = json.Unmarshal(raw, &got)
	if !got.ValidateOnly {
		t.Fatalf("cuerpo = %s", raw)
	}
}
