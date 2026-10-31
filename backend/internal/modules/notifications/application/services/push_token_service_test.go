package services_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/memory"
)

var registeredAt = time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

func newPushTokenService(repo ports.PushTokenRepository) *services.PushTokenService {
	return services.NewPushTokenService(repo, ports.ClockFunc(func() time.Time { return registeredAt }), testLogger())
}

func TestPushTokenService_RegisterStoresTheDevice(t *testing.T) {
	store := memory.NewPushDeviceStore()
	err := newPushTokenService(store).Register(context.Background(),
		services.RegisterPushTokenCommand{DeviceID: "CEL-1A2B3C", Token: "token-fcm", Platform: "ANDROID"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := store.List(context.Background())
	want := []entities.PushDevice{{DeviceID: "CEL-1A2B3C", Token: "token-fcm", Platform: entities.PushPlatformAndroid, UpdatedAt: registeredAt}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
}

type failingRepo struct {
	memory.PushDeviceStore
	upserts int
}

func (r *failingRepo) Upsert(context.Context, entities.PushDevice) error {
	r.upserts++
	return errors.New("base caída")
}

func TestPushTokenService_InvalidRegistrationDoesNotTouchTheRepository(t *testing.T) {
	repo := &failingRepo{}
	err := newPushTokenService(repo).Register(context.Background(),
		services.RegisterPushTokenCommand{DeviceID: "CEL-1", Token: "", Platform: "ANDROID"})
	if !errors.Is(err, entities.ErrInvalidPushDevice) {
		t.Fatalf("error = %v, se esperaba ErrInvalidPushDevice", err)
	}
	if repo.upserts != 0 {
		t.Fatalf("upserts = %d", repo.upserts)
	}
}

func TestPushTokenService_RepositoryErrorIsNotAValidationError(t *testing.T) {
	err := newPushTokenService(&failingRepo{}).Register(context.Background(),
		services.RegisterPushTokenCommand{DeviceID: "CEL-1", Token: "t", Platform: "ANDROID"})
	if err == nil || errors.Is(err, entities.ErrInvalidPushDevice) {
		t.Fatalf("error = %v, se esperaba un error de persistencia", err)
	}
}
