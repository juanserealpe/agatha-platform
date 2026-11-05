package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/memory"
)

func TestNewTestPushMessage_FollowsTheContract(t *testing.T) {
	now := time.Date(2026, 11, 3, 21, 4, 38, 0, time.UTC)
	got, err := NewTestPushMessage("red", "level_up", now)
	if err != nil {
		t.Fatal(err)
	}
	want := notificationEntities.PushMessage{
		MessageID: -now.UnixMilli(), AlertID: "ALR-PRUEBA-000001", Level: notificationEntities.PushLevelRed,
		Kind: notificationEntities.PushKindLevelUp, SensorID: "Sensor PK34-470", PK: "34+470",
		Event: notificationEntities.PushEventLeak, StartedAt: time.Date(2026, 11, 3, 21, 4, 0, 0, time.UTC),
	}
	if got != want {
		t.Fatalf("mensaje = %+v, se esperaba %+v", got, want)
	}
}

// Cada envío lleva otro messageId: la app no lo descarta como repetido.
func TestNewTestPushMessage_MessageIDChangesPerSend(t *testing.T) {
	now := time.Now()
	a, _ := NewTestPushMessage("RED", "NEW", now)
	b, _ := NewTestPushMessage("RED", "NEW", now.Add(time.Millisecond))
	if a.MessageID >= 0 || a.MessageID == b.MessageID {
		t.Fatalf("messageId = %d y %d", a.MessageID, b.MessageID)
	}
}

func TestNewTestPushMessage_RejectsUnknownValues(t *testing.T) {
	for _, c := range [][2]string{{"NORMAL", "NEW"}, {"ROJO", "NEW"}, {"RED", "DOWN"}, {"", ""}} {
		if _, err := NewTestPushMessage(c[0], c[1], time.Now()); err == nil {
			t.Fatalf("NewTestPushMessage(%q, %q) no falló", c[0], c[1])
		}
	}
}

func TestTestPushToken(t *testing.T) {
	store := memory.NewPushDeviceStore(
		notificationEntities.PushDevice{DeviceID: "CEL-1", Token: "token-1", Platform: notificationEntities.PushPlatformAndroid},
		notificationEntities.PushDevice{DeviceID: "CEL-2", Token: "token-2", Platform: notificationEntities.PushPlatformAndroid},
	)
	ctx := context.Background()

	if got, err := testPushToken(ctx, store, nil, " CEL-2 "); err != nil || got != "token-2" {
		t.Fatalf("sin allowlist: token = %q, error = %v", got, err)
	}
	if got, err := testPushToken(ctx, store, []string{"CEL-1"}, "CEL-1"); err != nil || got != "token-1" {
		t.Fatalf("en la allowlist: token = %q, error = %v", got, err)
	}
	if _, err := testPushToken(ctx, store, []string{"CEL-1"}, "CEL-2"); !errors.Is(err, ErrTestPushDeviceNotAllowed) {
		t.Fatalf("fuera de la allowlist: error = %v", err)
	}
	if _, err := testPushToken(ctx, store, nil, "CEL-9"); !errors.Is(err, ErrTestPushDeviceNotRegistered) {
		t.Fatalf("sin registro: error = %v", err)
	}
}
