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
// Engine guarda las filas correctas y publica una level_evaluated por cada
// hora de 24 h (la NO_CALCULABLE sin fila) en el bus.
func TestRuleEngine_EndToEndCrossingTheThreshold(t *testing.T) {
	const point int64 = 11556
	start := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	now := start.Add(48 * time.Hour)

	bus := eventbus.NewInMemory(nil)
	thresholds := memory.NewThresholdRepository(entities.ThresholdVersion{
		MeasuringPointID: point,
		EventType:        entities.EventTypeLeak,
		Version:          1,
		ThresholdParams:  referenceParams(),
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

	// Alert Management todavía no existe: el test escucha el tópico como lo hará él.
	levels := make(chan ruleEngineContract.LevelEvaluated, 32)
	eventbus.SubscribeTyped(bus, ruleEngineContract.TopicLevelEvaluated,
		func(_ context.Context, e ruleEngineContract.LevelEvaluated) { levels <- e }, nil)

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

	var got []ruleEngineContract.LevelEvaluated
	deadline := time.After(5 * time.Second)
	for len(got) < len(sequence) {
		select {
		case e := <-levels:
			got = append(got, e)
		case <-deadline:
			t.Fatalf("eventos recibidos: %d; se esperaban %d (uno por hora de 24 h)", len(got), len(sequence))
		}
	}

	closeBus(t, bus)

	if len(levels) != 0 {
		t.Fatalf("eventos de más: %d", len(levels))
	}

	// El bus entrega en orden: un evento por hora de 24 h, en la secuencia publicada.
	wantStatus := []string{"CALCULATED", "CALCULATED", "CALCULATED", "CALCULATED", "NO_CALCULABLE", "CALCULATED", "CALCULATED", "CALCULATED", "CALCULATED"}
	wantEvent := []string{"NONE", "NONE", "OPENED", "NONE", "", "NONE", "NONE", "CLEARED", "NONE"}
	wantEpisode := []string{"NORMAL", "NORMAL", "AMARILLO", "NARANJA", "", "NARANJA", "NARANJA", "NORMAL", "NORMAL"}
	for n, e := range got {
		if !e.EvaluatedAt.Equal(start.Add(time.Duration(n)*time.Hour)) || e.Status != wantStatus[n] ||
			e.EpisodeEvent != wantEvent[n] || e.EpisodeLevel != wantEpisode[n] ||
			e.EventType != "FUGA" || e.SchemaVersion != ruleEngineContract.SchemaVersion {
			t.Errorf("evento %d = %+v", n, e)
		}
	}
	if e := got[4]; e.RuleEvaluationID != 0 || e.IndicatorValue != nil || e.ComputedLevel != "" {
		t.Fatalf("NO_CALCULABLE = %+v: sin fila, sin I y sin niveles", e)
	}
	// HOLD: el episodio sigue en NARANJA pero el computed baja a NORMAL.
	if e := got[5]; e.ComputedLevel != "NORMAL" || !e.InEpisode || e.Exit.Amarillo != 0.16 {
		t.Fatalf("histéresis = %+v", e)
	}
	// 6 horas CALCULADAS desde la 0,19 (la hora NO_CALCULABLE no cuenta).
	if c := got[7]; c.HoursAbove != 6 || c.InEpisode {
		t.Fatalf("cierre = %+v", c)
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

// referenceParams son los parámetros del escenario de referencia de la
// especificación (amarillo 0,18, salida 0,16): los escenarios del test de punta a punta
// está escrito sobre esos cortes. La plantilla de producción
// (entities.DefaultThresholdParams) ya usa amarillo 0,16.
func referenceParams() entities.ThresholdParams {
	p := entities.DefaultThresholdParams()
	p.Amarillo = 0.18
	return p
}
