package events_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
)

func sampleIndicatorEvaluated() contract.IndicatorEvaluated {
	i := 0.42
	return contract.IndicatorEvaluated{
		MeasuringPointID: 11556,
		WindowHours:      24,
		EvaluatedAt:      time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Status:           "CALCULATED",
		I:                &i,
		LowConfidence:    true,
		Trigger:          "RECONCILER",
		OccurredAt:       time.Date(2026, 10, 15, 12, 35, 0, 0, time.UTC),
		SchemaVersion:    contract.SchemaVersion,
	}
}

func TestBusEventPublisher_PublishesOnIndicatorTopic(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	received := make(chan contract.IndicatorEvaluated, 1)
	eventbus.SubscribeTyped(bus, contract.TopicIndicatorEvaluated,
		func(_ context.Context, e contract.IndicatorEvaluated) { received <- e }, nil)

	want := sampleIndicatorEvaluated()
	if err := events.NewBusEventPublisher(bus).PublishIndicatorEvaluated(context.Background(), want); err != nil {
		t.Fatalf("PublishIndicatorEvaluated devolvió %v", err)
	}

	select {
	case got := <-received:
		if got.MeasuringPointID != want.MeasuringPointID || got.WindowHours != want.WindowHours ||
			!got.EvaluatedAt.Equal(want.EvaluatedAt) || got.I == nil || *got.I != *want.I {
			t.Fatalf("evento recibido = %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el suscriptor no recibió el evento")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bus.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestNoopEventPublisher_NeverFails(t *testing.T) {
	if err := events.NewNoopEventPublisher().PublishIndicatorEvaluated(context.Background(), sampleIndicatorEvaluated()); err != nil {
		t.Fatalf("el noop no debe fallar: %v", err)
	}
}

func TestIndicatorEvaluated_IsJSONSerializable(t *testing.T) {
	raw, err := json.Marshal(sampleIndicatorEvaluated())
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"measuring_point_id", "window_hours", "evaluated_at", "status", "no_calc_reason",
		"i", "low_confidence", "trigger", "occurred_at", "schema_version"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("falta el campo JSON %q en %s", key, raw)
		}
	}

	var roundTrip contract.IndicatorEvaluated
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.I == nil || *roundTrip.I != 0.42 || !roundTrip.EvaluatedAt.Equal(sampleIndicatorEvaluated().EvaluatedAt) {
		t.Fatalf("round-trip JSON falló: %+v", roundTrip)
	}
}
