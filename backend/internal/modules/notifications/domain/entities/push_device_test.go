package entities_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

func TestNewPushDevice_NormalizesTheRegistration(t *testing.T) {
	at := time.Date(2026, 10, 30, 9, 0, 0, 0, time.FixedZone("COT", -5*3600))
	got, err := entities.NewPushDevice(" CEL-1A2B3C ", " token-fcm ", "android", at)
	if err != nil {
		t.Fatal(err)
	}
	want := entities.PushDevice{DeviceID: "CEL-1A2B3C", Token: "token-fcm", Platform: entities.PushPlatformAndroid, UpdatedAt: at.UTC()}
	if got != want {
		t.Fatalf("dispositivo = %+v, se esperaba %+v", got, want)
	}
}

func TestNewPushDevice_RejectsInvalidRegistrations(t *testing.T) {
	cases := map[string]struct{ deviceID, token, platform string }{
		"sin deviceId":       {"  ", "token", "ANDROID"},
		"deviceId muy largo": {strings.Repeat("x", entities.MaxPushDeviceIDLength+1), "token", "ANDROID"},
		"sin token":          {"CEL-1", "", "ANDROID"},
		"token muy largo":    {"CEL-1", strings.Repeat("t", entities.MaxPushTokenLength+1), "ANDROID"},
		"plataforma iOS":     {"CEL-1", "token", "IOS"},
		"sin plataforma":     {"CEL-1", "token", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := entities.NewPushDevice(c.deviceID, c.token, c.platform, time.Now()); !errors.Is(err, entities.ErrInvalidPushDevice) {
				t.Fatalf("error = %v, se esperaba ErrInvalidPushDevice", err)
			}
		})
	}
}

// Los topes son inclusivos.
func TestNewPushDevice_AcceptsTheMaximumLengths(t *testing.T) {
	_, err := entities.NewPushDevice(strings.Repeat("x", entities.MaxPushDeviceIDLength),
		strings.Repeat("t", entities.MaxPushTokenLength), "ANDROID", time.Now())
	if err != nil {
		t.Fatal(err)
	}
}
