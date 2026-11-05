package bootstrap

import (
	"context"
	"slices"
	"testing"
	"time"

	alertPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	alertServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	alertEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	alertEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/events"
	alertMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
	notificationServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/services"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/fcm/fcmtest"
	notificationMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/memory"
	processingContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// pointNames es el directorio de dispositivos de Notifications en memoria.
type pointNames map[int64]string

func (p pointNames) DeviceName(_ context.Context, id int64) (string, error) { return p[id], nil }

// De punta a punta con el bus real: Processing → Rule Engine → Alert Management
// → Notifications. El push usa el MobilePushService real con un FCM falso; la
// web, un grabador en lugar del hub. Mismo escenario que
// TestAlertManagement_EndToEndFromProcessing, más una clasificación desde la web.
func TestNotifications_EndToEndFromProcessing(t *testing.T) {
	const point int64 = 11556
	start := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	log := logger.New("NotificationsTest", "Error")

	bus := eventbus.NewInMemory(nil)
	ruleEngine := wireRuleEngine(bus, ruleEngineDeps{
		thresholds: memory.NewThresholdRepository(entities.ThresholdVersion{
			MeasuringPointID: point, EventType: entities.EventTypeLeak, Version: 1, ThresholdParams: referenceParams(),
			ValidFrom: start.Add(-30 * 24 * time.Hour), Reason: entities.SeedReason, CreatedBy: entities.SystemCreatedBy,
		}),
		evaluations: memory.NewRuleEvaluationRepository(),
		clock:       ports.ClockFunc(func() time.Time { return start.Add(48 * time.Hour) }),
	}, log)
	ruleEngine.SubscribeIndicatorEvaluated()

	store := alertMemory.NewAlertStore()
	clock := alertPorts.ClockFunc(func() time.Time { return start.Add(time.Hour) })
	changes := busAlertChangePublisher{bus: bus}
	processor := alertServices.NewProcessingService(store, alertMemory.NewPolicyStore(), noDevices{},
		busAlertNotificationPublisher{bus: bus}, clock, log).WithChangePublisher(changes)
	subscribeLevelEvaluated(bus, alertEvents.NewLevelEvaluatedSubscriber(processor, log), log)
	management := alertServices.NewManagementService(store, clock, log).WithChangePublisher(changes)

	web := &recordingNotifications{}
	sender := fcmtest.NewFakeSender()
	push := notificationServices.NewMobilePushService(
		notificationMemory.NewPushDeviceStore(notificationEntities.PushDevice{
			DeviceID: "CEL-1", Token: "token-1", Platform: notificationEntities.PushPlatformAndroid, UpdatedAt: start,
		}),
		pointNames{point: "Sensor PK34-470"}, sender, notificationServices.MobilePushConfig{}, log)
	wireNotifications(bus, notificationChannels{webNotices: web, webChanges: web, push: push}, log).Subscribe()

	sequence := []float64{0.12, 0.19, 0.20, 0.30, -1 /* NO_CALCULABLE */, 0.17, 0.15, 0.14, 0.10}
	for n, v := range sequence {
		at := start.Add(time.Duration(n) * time.Hour)
		event := processingContract.IndicatorEvaluated{MeasuringPointID: point, WindowHours: 24, EvaluatedAt: at,
			Status: "CALCULATED", Trigger: "RECONCILER", OccurredAt: at.Add(35 * time.Minute), SchemaVersion: processingContract.SchemaVersion}
		if v < 0 {
			event.Status = "NO_CALCULABLE"
		} else {
			i := v
			event.I = &i
		}
		eventbus.PublishTyped(context.Background(), bus, processingContract.TopicIndicatorEvaluated, event)
	}

	// Cerrar el bus antes de tiempo descartaría lo que la cadena aún no publicó.
	webChanges := func() []string {
		web.mu.Lock()
		defer web.mu.Unlock()
		return slices.Clone(web.changes)
	}
	deadline := time.After(5 * time.Second)
	for {
		st, ok := store.State(point, alertEntities.EventTypeLeak)
		if ok && st.LastEvaluatedAt != nil && st.LastEvaluatedAt.Equal(start.Add(8*time.Hour)) &&
			slices.Contains(webChanges(), alertContract.ChangeFinished) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("Alert Management no procesó las 9 horas (estado %+v, cambios %v)", st, webChanges())
		case <-time.After(5 * time.Millisecond):
		}
	}

	alerts := store.Alerts()
	if len(alerts) != 1 {
		t.Fatalf("alertas = %d", len(alerts))
	}
	a := alerts[0]
	if _, err := management.Classify(context.Background(), alertServices.ClassifyCommand{
		AlertID: a.ID, Classification: alertEntities.ClassificationConfirmed,
		ActionOrigin: alertServices.ActionOrigin{Origin: alertEntities.OriginWeb},
	}); err != nil {
		t.Fatal(err)
	}
	closeBus(t, bus)

	// Web: todos los avisos y la foto de cada cambio, incluida la clasificación.
	if got := web.notices; !slices.Equal(got, []string{"OPEN", "UP_NARANJA_FAST"}) {
		t.Fatalf("avisos web = %v", got)
	}
	for _, want := range []string{alertContract.ChangeOpened, alertContract.ChangeLevelRaised, alertContract.ChangeNoDataStarted,
		alertContract.ChangeNoDataEnded, alertContract.ChangeFinished, alertContract.ChangeClassified} {
		if !slices.Contains(web.changes, want) {
			t.Errorf("falta el cambio %s en %v", want, web.changes)
		}
	}
	if last := web.changes[len(web.changes)-1]; last != alertContract.ChangeClassified {
		t.Errorf("último cambio = %s, se esperaba %s (%v)", last, alertContract.ChangeClassified, web.changes)
	}

	// Push: solo NEW y LEVEL_UP, al celular registrado, con el código y el PK.
	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("push enviados = %+v", sent)
	}
	for i, want := range []struct {
		kind  notificationEntities.PushKind
		level notificationEntities.PushLevel
	}{
		{notificationEntities.PushKindNew, notificationEntities.PushLevelYellow},
		{notificationEntities.PushKindLevelUp, notificationEntities.PushLevelOrange},
	} {
		s := sent[i]
		if s.Token != "token-1" || s.Message.Kind != want.kind || s.Message.Level != want.level ||
			s.Message.AlertID != a.Code || s.Message.PK != "34+470" || !s.Message.StartedAt.Equal(a.StartedAt) {
			t.Errorf("push %d = %+v", i, s)
		}
	}
}
