package bootstrap

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	alertPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	alertServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	alertEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	alertEvents "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/events"
	alertMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
	processingContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	ruleEngineContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestToLevelNotice_TranslatesTheContract(t *testing.T) {
	i, twelve := 0.22, 12
	n := toLevelNotice(ruleEngineContract.LevelEvaluated{
		RuleEvaluationID: 9, MeasuringPointID: 11556, EventType: "FUGA", EvaluatedAt: time.Date(2026, 8, 24, 22, 0, 0, 0, time.UTC),
		Status: "CALCULATED", IndicatorValue: &i, RawLevel: "AMARILLO", ComputedLevel: "NARANJA", EpisodeLevel: "NARANJA",
		InEpisode: true, EpisodeEvent: "NONE",
		Reasons: []ruleEngineContract.Reason{
			{Code: ruleEngineContract.ReasonLeakThreshold, Level: "AMARILLO"},
			{Code: ruleEngineContract.ReasonLeakPersistence, Level: "NARANJA", Data: ruleEngineContract.ReasonData{Hours: 12}},
		},
		Exit: ruleEngineContract.LevelCuts{Amarillo: 0.14}, Runs: ruleEngineContract.LevelRuns{Amarillo: 12},
		Escalation: ruleEngineContract.Escalation{AmarilloAfterHours: &twelve}, BelowRun: 0, HoursAbove: 13,
	})
	if n.PersistenceHours != 12 || n.ComputedLevel != alertEntities.LevelNaranja || n.Exit.Amarillo != 0.14 ||
		*n.EscalationAmarillo != 12 || n.RunsAmarillo != 12 || !n.Calculated() || n.RuleEvaluationID != 9 {
		t.Fatalf("notice = %+v", n)
	}
}

// De punta a punta con el bus real: Processing → Rule Engine → Alert Management
// → alert_management.alert_notification (lo que consumirá Notification).
func TestAlertManagement_EndToEndFromProcessing(t *testing.T) {
	const point int64 = 11556
	start := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	log := logger.New("AlertManagementTest", "Error")

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
	processor := alertServices.NewProcessingService(store, alertMemory.NewPolicyStore(), noDevices{},
		busAlertNotificationPublisher{bus: bus},
		alertPorts.ClockFunc(func() time.Time { return start.Add(time.Hour) }), log)
	subscribeLevelEvaluated(bus, alertEvents.NewLevelEvaluatedSubscriber(processor, log), log)

	var mu sync.Mutex
	var published []alertContract.AlertNotification
	eventbus.SubscribeTyped(bus, alertContract.TopicAlertNotification, func(_ context.Context, n alertContract.AlertNotification) {
		mu.Lock()
		defer mu.Unlock()
		published = append(published, n)
	}, log, eventbus.WithName("notification.test"))
	publishedCodes := func() []string {
		mu.Lock()
		defer mu.Unlock()
		var codes []string
		for _, n := range published {
			codes = append(codes, n.Code)
		}
		return codes
	}

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

	deadline := time.After(5 * time.Second)
	for {
		st, ok := store.State(point, alertEntities.EventTypeLeak)
		if ok && st.LastEvaluatedAt != nil && st.LastEvaluatedAt.Equal(start.Add(8*time.Hour)) && len(publishedCodes()) == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("Alert Management no procesó las 9 horas (estado %+v, avisos %v)", st, publishedCodes())
		case <-time.After(5 * time.Millisecond):
		}
	}
	closeBus(t, bus)

	alerts := store.Alerts()
	if len(alerts) != 1 {
		t.Fatalf("alertas = %d", len(alerts))
	}
	a := alerts[0]
	if a.State != alertEntities.AlertFinished || !a.StartedAt.Equal(start.Add(time.Hour)) || a.MaxLevel != alertEntities.LevelNaranja ||
		a.EndedAt == nil || !a.EndedAt.Equal(start.Add(7*time.Hour)) || a.HoursNoData != 1 {
		t.Fatalf("alerta = %+v", a)
	}
	var codes []string
	for _, m := range store.Messages() {
		codes = append(codes, m.Code)
	}
	for _, want := range []string{"OPEN", "UP_NARANJA_FAST", "NO_CALC_START", "NO_CALC_END", "END_NORMAL"} {
		if !slices.Contains(codes, want) {
			t.Errorf("falta %s en %v", want, codes)
		}
	}
	// Solo los mensajes NOTIFICACION salen hacia Notification, en orden.
	if got := publishedCodes(); !slices.Equal(got, []string{"OPEN", "UP_NARANJA_FAST"}) {
		t.Fatalf("avisos publicados = %v", got)
	}
	if n := published[0]; n.AlertCode != a.Code || n.Source != alertContract.SourceLive || n.Late || n.MessageID == 0 {
		t.Fatalf("aviso = %+v", n)
	}
}

type noDevices struct{}

func (noDevices) Get(context.Context, int64) (*alertEntities.Device, error) { return nil, nil }
