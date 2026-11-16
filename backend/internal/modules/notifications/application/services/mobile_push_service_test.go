package services_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/fcm/fcmtest"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/memory"
)

type devicesByPoint map[int64]string

func (d devicesByPoint) DeviceName(_ context.Context, id int64) (string, error) { return d[id], nil }

type brokenDevices struct{}

func (brokenDevices) DeviceName(context.Context, int64) (string, error) {
	return "", errors.New("base caída")
}

// waits graba las esperas entre reintentos sin esperar de verdad.
type waits struct {
	mu  sync.Mutex
	got []time.Duration
}

func (w *waits) sleep(_ context.Context, d time.Duration) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.got = append(w.got, d)
	return nil
}

func phone(id, token string) entities.PushDevice {
	return entities.PushDevice{DeviceID: id, Token: token, Platform: entities.PushPlatformAndroid, UpdatedAt: registeredAt}
}

func notice() entities.AlertNotice {
	return entities.AlertNotice{MessageID: 9, AlertCode: "ALR-000142", MeasuringPointID: 11556, Code: "UP_ROJO",
		Level: entities.LevelRojo, EventType: entities.EventLeak}
}

type pushFixture struct {
	store  *memory.PushDeviceStore
	sender *fcmtest.FakeSender
	waits  *waits
	svc    *services.MobilePushService
}

func newPushFixture(devices ports.DeviceDirectory, cfg services.MobilePushConfig, phones ...entities.PushDevice) pushFixture {
	f := pushFixture{store: memory.NewPushDeviceStore(phones...), sender: fcmtest.NewFakeSender(), waits: &waits{}}
	cfg.Sleep = f.waits.sleep
	f.svc = services.NewMobilePushService(f.store, devices, f.sender, cfg, testLogger())
	return f
}

func TestMobilePushService_SendsToEveryPhone(t *testing.T) {
	f := newPushFixture(devicesByPoint{11556: "Sensor PK34-470"}, services.MobilePushConfig{},
		phone("CEL-1", "a"), phone("CEL-2", "b"), phone("CEL-3", "c"))
	if err := f.svc.DeliverNotice(context.Background(), notice()); err != nil {
		t.Fatal(err)
	}
	sent := f.sender.Sent()
	if len(sent) != 3 || f.sender.Attempts("a") != 1 || f.sender.Attempts("b") != 1 || f.sender.Attempts("c") != 1 {
		t.Fatalf("envíos = %+v: se esperaba uno por token", sent)
	}
	want := entities.PushMessage{MessageID: 9, AlertID: "ALR-000142", Level: entities.PushLevelRed, Kind: entities.PushKindLevelUp,
		SensorID: "Sensor PK34-470", PK: "34+470", Event: entities.PushEventLeak}
	if sent[0].Message != want {
		t.Fatalf("mensaje = %+v, se esperaba %+v", sent[0].Message, want)
	}
}

// Sin el nombre del dispositivo el push sale igual, sin sensorId ni pk.
func TestMobilePushService_DeviceLookupErrorStillSends(t *testing.T) {
	f := newPushFixture(brokenDevices{}, services.MobilePushConfig{}, phone("CEL-1", "a"))
	if err := f.svc.DeliverNotice(context.Background(), notice()); err != nil {
		t.Fatal(err)
	}
	if sent := f.sender.Sent(); len(sent) != 1 || sent[0].Message.SensorID != "" || sent[0].Message.PK != "" {
		t.Fatalf("envíos = %+v", sent)
	}
}

func TestMobilePushService_RetriesWithBackoff(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{}, phone("CEL-1", "a"))
	f.sender.FailNext("a",
		&ports.RetryablePushError{Err: errors.New("503")},
		&ports.RetryablePushError{Err: errors.New("red")},
		&ports.RetryablePushError{Err: errors.New("500")})
	if err := f.svc.DeliverNotice(context.Background(), notice()); err != nil {
		t.Fatal(err)
	}
	if n := f.sender.Attempts("a"); n != 4 {
		t.Fatalf("intentos = %d", n)
	}
	if want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}; !reflect.DeepEqual(f.waits.got, want) {
		t.Fatalf("esperas = %v, se esperaba %v", f.waits.got, want)
	}
}

// Retry-After manda si pide más que el backoff, con tope.
func TestMobilePushService_RespectsRetryAfterWithACap(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{}, phone("CEL-1", "a"))
	f.sender.FailNext("a",
		&ports.RetryablePushError{RetryAfter: 7 * time.Second, Err: errors.New("429")},
		&ports.RetryablePushError{RetryAfter: time.Hour, Err: errors.New("429")})
	_ = f.svc.DeliverNotice(context.Background(), notice())
	want := []time.Duration{7 * time.Second, services.DefaultPushMaxRetryAfter}
	if !reflect.DeepEqual(f.waits.got, want) {
		t.Fatalf("esperas = %v, se esperaba %v", f.waits.got, want)
	}
}

func TestMobilePushService_StopsAtMaxAttempts(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{MaxAttempts: 2}, phone("CEL-1", "a"), phone("CEL-2", "b"))
	f.sender.FailNext("a", &ports.RetryablePushError{Err: errors.New("503")}, &ports.RetryablePushError{Err: errors.New("503")})
	if err := f.svc.DeliverNotice(context.Background(), notice()); err == nil {
		t.Fatal("se esperaba error")
	}
	if f.sender.Attempts("a") != 2 || f.sender.Attempts("b") != 1 {
		t.Fatalf("intentos a=%d b=%d: un celular que falla no frena a los demás", f.sender.Attempts("a"), f.sender.Attempts("b"))
	}
}

func TestMobilePushService_DoesNotRetryPermanentErrors(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{}, phone("CEL-1", "a"))
	f.sender.FailNext("a", errors.New("FCM rechazó el mensaje (400)"))
	if err := f.svc.DeliverNotice(context.Background(), notice()); err == nil {
		t.Fatal("se esperaba error")
	}
	if n := f.sender.Attempts("a"); n != 1 || len(f.waits.got) != 0 {
		t.Fatalf("intentos = %d, esperas = %v", n, f.waits.got)
	}
	if got, _ := f.store.List(context.Background()); len(got) != 1 {
		t.Fatal("un error definitivo no borra el token")
	}
}

func TestMobilePushService_RemovesUnregisteredTokens(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{},
		phone("CEL-1", "a"), phone("CEL-2", "b"), phone("CEL-3", "c"))
	f.sender.FailNext("a", ports.ErrPushTokenUnregistered)
	if err := f.svc.DeliverNotice(context.Background(), notice()); err != nil {
		t.Fatalf("un token borrado no es una falla: %v", err)
	}
	got, _ := f.store.List(context.Background())
	if want := []entities.PushDevice{phone("CEL-2", "b"), phone("CEL-3", "c")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("celulares = %+v, se esperaba %+v", got, want)
	}
	if f.sender.Attempts("a") != 1 {
		t.Fatal("UNREGISTERED no se reintenta")
	}
	if f.sender.Attempts("b") != 1 || f.sender.Attempts("c") != 1 {
		t.Fatal("un token borrado no frena el envío a los demás")
	}
}

func TestMobilePushService_AllowlistLimitsTheRecipients(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{Allowlist: []string{"CEL-PRUEBA"}},
		phone("CEL-1", "a"), phone("CEL-PRUEBA", "p"))
	if err := f.svc.DeliverNotice(context.Background(), notice()); err != nil {
		t.Fatal(err)
	}
	if sent := f.sender.Sent(); len(sent) != 1 || sent[0].Token != "p" {
		t.Fatalf("envíos = %+v", sent)
	}
}

func TestMobilePushService_NoPhonesIsNotAnError(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{})
	if err := f.svc.DeliverNotice(context.Background(), notice()); err != nil {
		t.Fatal(err)
	}
}

// Un aviso que no se puede enviar por push (la política debió descartarlo) no llega a FCM.
func TestMobilePushService_RejectsNonPushableNotices(t *testing.T) {
	f := newPushFixture(devicesByPoint{}, services.MobilePushConfig{}, phone("CEL-1", "a"))
	n := notice()
	n.Level = "NORMAL"
	if err := f.svc.DeliverNotice(context.Background(), n); !errors.Is(err, entities.ErrNotPushable) {
		t.Fatalf("error = %v", err)
	}
	if len(f.sender.Sent()) != 0 {
		t.Fatal("no debía enviar")
	}
}

// Si el bus corta el plazo, los reintentos paran.
func TestMobilePushService_StopsRetryingWhenTheContextEnds(t *testing.T) {
	store := memory.NewPushDeviceStore(phone("CEL-1", "a"))
	sender := fcmtest.NewFakeSender()
	sender.FailNext("a", &ports.RetryablePushError{Err: errors.New("503")}, &ports.RetryablePushError{Err: errors.New("503")})
	ctx, cancel := context.WithCancel(context.Background())
	svc := services.NewMobilePushService(store, devicesByPoint{}, sender, services.MobilePushConfig{
		Sleep: func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() },
	}, testLogger())
	if err := svc.DeliverNotice(ctx, notice()); err == nil {
		t.Fatal("se esperaba error")
	}
	if n := sender.Attempts("a"); n != 1 {
		t.Fatalf("intentos = %d", n)
	}
}
