package bootstrap

import (
	"context"
	"reflect"
	"testing"
	"time"

	notificationPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// capturingChannels graba los avisos y cambios completos.
type capturingChannels struct {
	recordingNotifications
	lastNotice notificationEntities.AlertNotice
	lastChange notificationEntities.AlertChange
}

func (c *capturingChannels) DeliverNotice(ctx context.Context, n notificationEntities.AlertNotice) error {
	c.mu.Lock()
	c.lastNotice = n
	c.mu.Unlock()
	return c.recordingNotifications.DeliverNotice(ctx, n)
}

func (c *capturingChannels) DeliverChange(ctx context.Context, ch notificationEntities.AlertChange) error {
	c.mu.Lock()
	c.lastChange = ch
	c.mu.Unlock()
	return c.recordingNotifications.DeliverChange(ctx, ch)
}

var testEventNow = time.Date(2026, 10, 30, 14, 25, 0, 0, time.UTC)

func emitTestEvents(t *testing.T, events ...notificationEntities.TestEvent) (web, push *capturingChannels, results []notificationPorts.TestEventResult) {
	t.Helper()
	bus := eventbus.NewInMemory(nil)
	web, push = &capturingChannels{}, &capturingChannels{}
	wireNotifications(bus, notificationChannels{webNotices: web, webChanges: web, push: push},
		logger.New("NotificationsTest", "Error")).Subscribe()
	emitter := newBusTestEventEmitter(bus, notificationPorts.ClockFunc(func() time.Time { return testEventNow }))
	for _, ev := range events {
		valid, err := notificationEntities.NewTestEvent(ev)
		if err != nil {
			t.Fatal(err)
		}
		r, err := emitter.Emit(context.Background(), valid)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, r)
	}
	closeBus(t, bus)
	return web, push, results
}

// Un aviso de prueba recorre el pipeline real: la web recibe el aviso y su
// cambio, y el push lo recibe detrás de su política.
func TestTestEventEmitter_GoesThroughEveryChannel(t *testing.T) {
	web, push, results := emitTestEvents(t, notificationEntities.TestEvent{MeasuringPointID: 11556, Level: "rojo"})

	r := results[0]
	if r.AlertID >= 0 || r.MessageID >= 0 || r.AlertCode != "ALR-PRUEBA-000001" || r.Change != notificationEntities.ChangeOpened {
		t.Fatalf("resultado = %+v: los ids de prueba son negativos", r)
	}
	if !reflect.DeepEqual(web.notices, []string{"OPEN"}) || !reflect.DeepEqual(web.changes, []string{"OPENED"}) {
		t.Fatalf("web recibió avisos %v y cambios %v", web.notices, web.changes)
	}
	if !reflect.DeepEqual(push.notices, []string{"OPEN"}) {
		t.Fatalf("push recibió %v", push.notices)
	}
	c := web.lastChange
	if c.AlertID != r.AlertID || c.MeasuringPointID != 11556 || c.State != "ACTIVA" || c.CurrentLevel != "ROJO" ||
		!c.Blinking || c.ManagementStatus != "SIN_CLASIFICAR" || !c.EvaluatedAt.Equal(testEventNow.Truncate(time.Hour)) {
		t.Fatalf("cambio = %+v", c)
	}
	if n := web.lastNotice; n.MessageID != r.MessageID || n.EventType != "FUGA" || n.Title == "" {
		t.Fatalf("aviso = %+v", n)
	}
}

// Repetir sobre la misma alerta (subida de nivel) y probar las reglas del push.
func TestTestEventEmitter_SameAlertAndPushPolicy(t *testing.T) {
	_, push, results := emitTestEvents(t,
		notificationEntities.TestEvent{MeasuringPointID: 1, Level: "AMARILLO"},
		notificationEntities.TestEvent{MeasuringPointID: 1, AlertID: -1, Level: "ROJO", Code: "UP_ROJO", Late: true},
		notificationEntities.TestEvent{MeasuringPointID: 1, AlertID: -1, Level: "ROJO", Code: "UP_ROJO", Classification: "FALSA_ALERTA"},
	)
	if results[1].AlertID != results[0].AlertID || results[1].MessageID == results[0].MessageID {
		t.Fatalf("resultados = %+v: misma alerta, mensajes distintos", results)
	}
	if !reflect.DeepEqual(push.notices, []string{"OPEN"}) {
		t.Fatalf("push recibió %v: omite atrasados y falsas alertas", push.notices)
	}
}

// Un cambio silencioso solo actualiza el mapa y el panel.
func TestTestEventEmitter_SilentChange(t *testing.T) {
	web, push, results := emitTestEvents(t,
		notificationEntities.TestEvent{MeasuringPointID: 1, AlertID: -3, Level: "ROJO", Change: "classified", Classification: "CONFIRMADA"})
	if results[0].MessageID != 0 || len(web.notices) != 0 || len(push.notices) != 0 {
		t.Fatalf("resultado %+v, avisos web %v, push %v: un cambio no avisa", results[0], web.notices, push.notices)
	}
	if c := web.lastChange; c.Change != "CLASSIFIED" || c.Blinking || c.ManagementStatus != "CLASIFICADA_SIN_ETIQUETA" {
		t.Fatalf("cambio = %+v: clasificada deja de titilar", c)
	}
}

func TestTestEndpointEnabled(t *testing.T) {
	log := logger.New("NotificationsTest", "Error")
	cases := []struct {
		flag, env string
		want      bool
	}{
		{"", "", false},
		{"false", "development", false},
		{"true", "development", true},
		{"true", "", true},
		{"true", "production", false},
		// Mismo criterio que el -rebuild de Rule Engine y Alert Management.
		{"true", "Production", false},
		{"true", "PRODUCTION", false},
		{"true", " production ", false},
	}
	for _, c := range cases {
		t.Setenv(NotificationsTestEndpointEnv, c.flag)
		t.Setenv(EnvironmentEnv, c.env)
		if got := testEndpointEnabled(log); got != c.want {
			t.Errorf("flag=%q env=%q: habilitado = %v, se esperaba %v", c.flag, c.env, got, c.want)
		}
	}
}
