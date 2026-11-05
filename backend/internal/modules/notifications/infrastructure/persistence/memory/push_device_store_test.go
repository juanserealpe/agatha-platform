package memory_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/memory"
)

var _ ports.PushTokenRepository = (*memory.PushDeviceStore)(nil)

var at = time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

func device(id, token string) entities.PushDevice {
	return entities.PushDevice{DeviceID: id, Token: token, Platform: entities.PushPlatformAndroid, UpdatedAt: at}
}

func list(t *testing.T, s *memory.PushDeviceStore) []entities.PushDevice {
	t.Helper()
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestPushDeviceStore_UpsertIsIdempotent(t *testing.T) {
	s := memory.NewPushDeviceStore()
	for range 2 {
		if err := s.Upsert(context.Background(), device("CEL-1", "a")); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := list(t, s), []entities.PushDevice{device("CEL-1", "a")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
}

// Un token nuevo reemplaza al anterior del mismo celular.
func TestPushDeviceStore_UpsertReplacesTheDeviceToken(t *testing.T) {
	s := memory.NewPushDeviceStore(device("CEL-1", "viejo"), device("CEL-2", "b"))
	if err := s.Upsert(context.Background(), device("CEL-1", "nuevo")); err != nil {
		t.Fatal(err)
	}
	want := []entities.PushDevice{device("CEL-1", "nuevo"), device("CEL-2", "b")}
	if got := list(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
}

// Un token que ya era de otro celular (p. ej., la app se reinstaló con otro
// deviceId) pasa al nuevo: nunca se envía dos veces al mismo token.
func TestPushDeviceStore_UpsertReassignsAToken(t *testing.T) {
	s := memory.NewPushDeviceStore(device("CEL-1", "a"))
	if err := s.Upsert(context.Background(), device("CEL-9", "a")); err != nil {
		t.Fatal(err)
	}
	if got, want := list(t, s), []entities.PushDevice{device("CEL-9", "a")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
}

func TestPushDeviceStore_DeleteTokenRemovesOnlyThatToken(t *testing.T) {
	s := memory.NewPushDeviceStore(device("CEL-1", "a"), device("CEL-2", "b"))
	if err := s.DeleteToken(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	// Un token que ya no está (el celular registró otro) no es un error.
	if err := s.DeleteToken(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if got, want := list(t, s), []entities.PushDevice{device("CEL-2", "b")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
}
