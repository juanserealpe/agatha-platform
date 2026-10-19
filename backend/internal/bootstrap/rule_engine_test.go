package bootstrap

import (
	"context"
	"testing"
	"time"

	processingContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	ruleEngineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	ruleEngineContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type noticeRecorder chan ruleEngineServices.IndicatorEvaluatedNotice

func (r noticeRecorder) Handle(_ context.Context, n ruleEngineServices.IndicatorEvaluatedNotice) {
	r <- n
}

// Processing publica su contrato y el Rule Engine recibe su propio Notice
// traducido por bootstrap.
func TestSubscribeRuleEngine_TranslatesIndicatorEvaluated(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	received := make(noticeRecorder, 1)
	subscribeIndicatorEvaluated(bus, received, logger.New("RuleEngineTest", "Error"))

	i := 0.31
	event := processingContract.IndicatorEvaluated{
		MeasuringPointID: 11556,
		WindowHours:      6,
		EvaluatedAt:      time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Status:           "CALCULATED",
		NoCalcReason:     "",
		I:                &i,
		LowConfidence:    true,
		Trigger:          "RECONCILER",
		OccurredAt:       time.Date(2026, 10, 15, 12, 35, 0, 0, time.UTC),
		SchemaVersion:    processingContract.SchemaVersion,
	}
	eventbus.PublishTyped(context.Background(), bus, processingContract.TopicIndicatorEvaluated, event)

	select {
	case got := <-received:
		want := ruleEngineServices.IndicatorEvaluatedNotice{
			MeasuringPointID: event.MeasuringPointID,
			WindowHours:      event.WindowHours,
			EvaluatedAt:      event.EvaluatedAt,
			Status:           event.Status,
			NoCalcReason:     event.NoCalcReason,
			I:                event.I,
			LowConfidence:    event.LowConfidence,
			Trigger:          event.Trigger,
			OccurredAt:       event.OccurredAt,
		}
		if got != want {
			t.Fatalf("notice = %+v, se esperaba %+v", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el Rule Engine no recibió el evento")
	}

	closeBus(t, bus)

	for _, s := range bus.Stats() {
		if s.Topic == processingContract.TopicIndicatorEvaluated && s.Name != indicatorEvaluatedSubscription {
			t.Errorf("nombre de suscripción = %q", s.Name)
		}
	}
}

// De punta a punta con el bus real: Processing publica una secuencia que cruza
// el umbral (con ventanas de 6 h y un NO_CALCULABLE intercalados) ⇒ el Rule
// Engine guarda las filas correctas y publica exceeded/cleared en el bus.
func TestRuleEngine_EndToEndCrossingTheThreshold(t *testing.T) {
	const point int64 = 11556
	start := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	now := start.Add(48 * time.Hour)

	bus := eventbus.NewInMemory(nil)
	thresholds := memory.NewThresholdRepository(entities.ThresholdVersion{
		MeasuringPointID: point,
		EventType:        entities.EventTypeLeak,
		Version:          1,
		ThresholdParams:  entities.DefaultThresholdParams(),
		ValidFrom:        start.Add(-30 * 24 * time.Hour),
		Reason:           entities.SeedReason,
		CreatedBy:        entities.SystemCreatedBy,
	})
	evaluations := memory.NewRuleEvaluationRepository()

	module := wireRuleEngine(bus, ruleEngineDeps{
		thresholds:  thresholds,
		evaluations: evaluations,
		clock:       ports.ClockFunc(func() time.Time { return now }),
	}, logger.New("RuleEngineTest", "Error"))
	module.SubscribeIndicatorEvaluated()

	// Alert Management todavía no existe: el test escucha los tópicos como lo hará él.
	exceeded := make(chan ruleEngineContract.LeakThresholdExceeded, 16)
	cleared := make(chan ruleEngineContract.LeakThresholdCleared, 16)
	eventbus.SubscribeTyped(bus, ruleEngineContract.TopicLeakThresholdExceeded,
		func(_ context.Context, e ruleEngineContract.LeakThresholdExceeded) { exceeded <- e }, nil)
	eventbus.SubscribeTyped(bus, ruleEngineContract.TopicLeakThresholdCleared,
		func(_ context.Context, e ruleEngineContract.LeakThresholdCleared) { cleared <- e }, nil)

	publish := func(at time.Time, window int, status string, i *float64) {
		eventbus.PublishTyped(context.Background(), bus, processingContract.TopicIndicatorEvaluated, processingContract.IndicatorEvaluated{
			MeasuringPointID: point, WindowHours: window, EvaluatedAt: at, Status: status, I: i,
			Trigger: "RECONCILER", OccurredAt: at.Add(35 * time.Minute), SchemaVersion: processingContract.SchemaVersion,
		})
	}

	sequence := []float64{0.12, 0.19, 0.20, 0.30, -1 /* NO_CALCULABLE */, 0.17, 0.15, 0.14, 0.10}
	for n, v := range sequence {
		at := start.Add(time.Duration(n) * time.Hour)
		six := 0.99
		publish(at, 6, "CALCULATED", &six) // la de 6 h se ignora
		if v < 0 {
			publish(at, 24, "NO_CALCULABLE", nil)
			continue
		}
		i := v
		publish(at, 24, "CALCULATED", &i)
	}

	var gotExceeded []ruleEngineContract.LeakThresholdExceeded
	var gotCleared []ruleEngineContract.LeakThresholdCleared
	deadline := time.After(5 * time.Second)
	for len(gotExceeded) < 2 || len(gotCleared) < 1 {
		select {
		case e := <-exceeded:
			gotExceeded = append(gotExceeded, e)
		case c := <-cleared:
			gotCleared = append(gotCleared, c)
		case <-deadline:
			t.Fatalf("eventos recibidos: exceeded=%d cleared=%d; se esperaban 2 y 1", len(gotExceeded), len(gotCleared))
		}
	}

	closeBus(t, bus)

	if len(exceeded)+len(cleared) != 0 {
		t.Fatalf("eventos de más: exceeded=%d cleared=%d", len(exceeded), len(cleared))
	}

	if e := gotExceeded[0]; e.Level != "AMARILLO" || e.PreviousLevel != "NORMAL" || e.HoursAbove != 2 ||
		!e.EvaluatedAt.Equal(start.Add(2*time.Hour)) || e.EventType != "FUGA" || e.SchemaVersion != ruleEngineContract.SchemaVersion {
		t.Fatalf("primer exceeded = %+v", e)
	}
	if e := gotExceeded[1]; e.Level != "NARANJA" || e.PreviousLevel != "AMARILLO" {
		t.Fatalf("segundo exceeded = %+v", e)
	}
	// 6 horas CALCULADAS desde la 0,19 (la hora NO_CALCULABLE no cuenta).
	if c := gotCleared[0]; c.LastLevel != "NARANJA" || !c.EvaluatedAt.Equal(start.Add(7*time.Hour)) || c.HoursAbove != 6 {
		t.Fatalf("cleared = %+v", c)
	}

	rows := evaluations.Rows(point)
	wantLevels := []entities.Level{"NORMAL", "NORMAL", "AMARILLO", "NARANJA", "NARANJA", "NARANJA", "NORMAL", "NORMAL"}
	wantRules := []entities.Rule{"NONE", "NONE", "THRESHOLD", "THRESHOLD", "HOLD", "HOLD", "NONE", "NONE"}
	if len(rows) != len(wantLevels) {
		t.Fatalf("filas = %d, se esperaban %d (sin la hora NO_CALCULABLE ni las de 6 h)", len(rows), len(wantLevels))
	}
	for n, row := range rows {
		if row.Level != wantLevels[n] || row.Rule != wantRules[n] {
			t.Errorf("fila %d (%v): %s/%s, se esperaba %s/%s", n, row.EvaluatedAt, row.Level, row.Rule, wantLevels[n], wantRules[n])
		}
	}
}

func closeBus(t *testing.T, bus *eventbus.InMemoryBus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bus.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
