package mappers

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

func TestPushDevice_RoundTrip(t *testing.T) {
	d := entities.PushDevice{
		DeviceID: "CEL-1A2B3C", Token: "token-fcm", Platform: entities.PushPlatformAndroid,
		UpdatedAt: time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC),
	}
	m := ToPushDeviceModel(d)
	if m.Platform != "ANDROID" {
		t.Fatalf("plataforma = %q", m.Platform)
	}
	if got := ToPushDevice(m); got != d {
		t.Fatalf("ida y vuelta:\n got %+v\nwant %+v", got, d)
	}
}
