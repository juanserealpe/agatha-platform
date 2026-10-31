package bootstrap

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/realtime"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestToAlertNotice_TranslatesTheContract(t *testing.T) {
	started := time.Date(2026, 10, 18, 19, 0, 0, 0, time.UTC)
	evaluated := started.Add(2 * time.Hour)
	occurred := evaluated.Add(35 * time.Minute)
	got := toAlertNotice(alertContract.AlertNotification{
		MessageID: 9, AlertID: 142, AlertCode: "ALR-000142", MeasuringPointID: 11556, EventType: "FUGA",
		StartedAt: started, EvaluatedAt: evaluated, Code: "UP_ROJO", Level: "ROJO", MaxLevel: "ROJO",
		AlertState: "ACTIVA", Classification: "CONFIRMADA", Title: "t", Body: "b", Source: "LIVE",
		Late: true, PolicyVersionID: 3, OccurredAt: occurred, SchemaVersion: 1,
	})
	want := notificationEntities.AlertNotice{
		MessageID: 9, AlertID: 142, AlertCode: "ALR-000142", MeasuringPointID: 11556, EventType: "FUGA",
		StartedAt: started, EvaluatedAt: evaluated, Code: "UP_ROJO", Level: "ROJO", MaxLevel: "ROJO",
		Classification: "CONFIRMADA", Title: "t", Body: "b", Late: true, OccurredAt: occurred,
	}
	if got != want {
		t.Fatalf("aviso = %+v, se esperaba %+v", got, want)
	}
}

func TestToAlertChange_TranslatesTheContract(t *testing.T) {
	started := time.Date(2026, 10, 18, 19, 0, 0, 0, time.UTC)
	evaluated := started.Add(2 * time.Hour)
	occurred := evaluated.Add(35 * time.Minute)
	got := toAlertChange(alertContract.AlertChanged{
		AlertID: 142, AlertCode: "ALR-000142", MeasuringPointID: 11556, EventType: "FUGA", Change: "CLASSIFIED",
		State: "ACTIVA", CurrentLevel: "ROJO", MaxLevel: "ROJO", Blinking: true, ManagementStatus: "CLASIFICADA_SIN_ETIQUETA",
		Classification: "CONFIRMADA", StartedAt: started, EvaluatedAt: evaluated, OccurredAt: occurred, SchemaVersion: 1,
	})
	want := notificationEntities.AlertChange{
		AlertID: 142, AlertCode: "ALR-000142", MeasuringPointID: 11556, EventType: "FUGA", Change: "CLASSIFIED",
		State: "ACTIVA", CurrentLevel: "ROJO", MaxLevel: "ROJO", Blinking: true, ManagementStatus: "CLASIFICADA_SIN_ETIQUETA",
		Classification: "CONFIRMADA", StartedAt: started, EvaluatedAt: evaluated, OccurredAt: occurred,
	}
	if got != want {
		t.Fatalf("cambio = %+v, se esperaba %+v", got, want)
	}
}

// recordingNotifications graba lo que recibe un canal (los workers del bus
// corren en paralelo).
type recordingNotifications struct {
	mu      sync.Mutex
	notices []string
	changes []string
}

func (r *recordingNotifications) DeliverNotice(_ context.Context, n notificationEntities.AlertNotice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, n.Code)
	return nil
}

func (r *recordingNotifications) DeliverChange(_ context.Context, c notificationEntities.AlertChange) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.changes = append(r.changes, c.Change)
	return nil
}

// Con los adaptadores reales de Alert Management y el bus real: cada canal
// recibe lo que su política permite.
func TestNotifications_DeliversEachChannelThroughTheBus(t *testing.T) {
	log := logger.New("NotificationsTest", "Error")
	bus := eventbus.NewInMemory(nil)
	web, push := &recordingNotifications{}, &recordingNotifications{}
	wireNotifications(bus, notificationChannels{webNotices: web, webChanges: web, push: push}, log).Subscribe()

	notices := busAlertNotificationPublisher{bus: bus}
	for _, n := range []alertContract.AlertNotification{
		{AlertCode: "ALR-000142", Code: "OPEN", Level: "AMARILLO"},
		{AlertCode: "ALR-000142", Code: "UP_ROJO", Level: "ROJO", Late: true},
		{AlertCode: "ALR-000142", Code: "UP_NARANJA_FAST", Level: "NARANJA", Classification: "FALSA_ALERTA"},
	} {
		if err := notices.PublishAlertNotification(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}
	if err := (busAlertChangePublisher{bus: bus}).PublishAlertChanged(context.Background(),
		alertContract.AlertChanged{AlertCode: "ALR-000142", Change: alertContract.ChangeClassified}); err != nil {
		t.Fatal(err)
	}
	closeBus(t, bus)

	if got := web.notices; len(got) != 3 {
		t.Fatalf("web recibió %v: recibe todos los avisos", got)
	}
	if got := web.changes; len(got) != 1 || got[0] != alertContract.ChangeClassified {
		t.Fatalf("web recibió los cambios %v", got)
	}
	if got := push.notices; len(got) != 1 || got[0] != "OPEN" {
		t.Fatalf("push recibió %v: omite atrasados y falsas alertas", got)
	}
	if len(push.changes) != 0 {
		t.Fatalf("push recibió cambios %v: solo los usa la web", push.changes)
	}
}

// Sin canales no hay suscripciones: publicar sigue siendo un no-op.
func TestNotifications_WithoutChannelsSubscribesNothing(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	unsubscribe := wireNotifications(bus, notificationChannels{}, logger.New("NotificationsTest", "Error")).Subscribe()
	defer unsubscribe()
	if err := (busAlertNotificationPublisher{bus: bus}).PublishAlertNotification(context.Background(),
		alertContract.AlertNotification{Code: "OPEN"}); err != nil {
		t.Fatal(err)
	}
	closeBus(t, bus)
}

// Del bus al navegador: un alert_changed publicado por Alert Management sale
// por GET /api/notifications/stream.
func TestNotifications_StreamsBusEventsOverSSE(t *testing.T) {
	log := logger.New("NotificationsTest", "Error")
	bus := eventbus.NewInMemory(nil)
	defer closeBus(t, bus)
	hub := realtime.NewHub(realtime.HubConfig{}, log)
	m := wireNotifications(bus, notificationChannels{webNotices: hub, webChanges: hub}, log)
	m.hub = hub
	defer m.Subscribe()()

	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/notifications/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	for hub.Clients() == 0 {
		time.Sleep(5 * time.Millisecond)
	}

	if err := (busAlertChangePublisher{bus: bus}).PublishAlertChanged(context.Background(),
		alertContract.AlertChanged{AlertCode: "ALR-000142", Change: alertContract.ChangeClassified}); err != nil {
		t.Fatal(err)
	}

	timeout := time.After(2 * time.Second)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("el stream se cerró")
			}
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"change":"CLASSIFIED"`) {
				return
			}
		case <-timeout:
			t.Fatal("el cambio no llegó al stream")
		}
	}
}
