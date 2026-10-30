package events_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/events"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func sampleEvent() contract.MeasurementsIngested {
	return contract.MeasurementsIngested{
		MeasuringPointID: 11556,
		Source:           "dataos",
		From:             time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC),
		To:               time.Date(2026, 9, 28, 15, 10, 0, 0, time.UTC),
		Watermark:        time.Date(2026, 9, 28, 15, 10, 0, 0, time.UTC),
		BatchSeq:         42,
		InsertedRecords:  3,
		OccurredAt:       time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC),
		SchemaVersion:    contract.SchemaVersion,
	}
}

func TestBusEventPublisher_PublishesOnIngestionTopic(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	received := make(chan contract.MeasurementsIngested, 1)
	eventbus.SubscribeTyped(bus, contract.TopicMeasurementsIngested,
		func(_ context.Context, e contract.MeasurementsIngested) { received <- e }, nil)

	publisher := events.NewBusEventPublisher(bus)
	if err := publisher.PublishMeasurementsIngested(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("PublishMeasurementsIngested devolvió %v", err)
	}

	select {
	case got := <-received:
		if got != sampleEvent() {
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

func TestMeasurementsIngested_IsJSONSerializable(t *testing.T) {
	raw, err := json.Marshal(sampleEvent())
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"measuring_point_id", "source", "from", "to", "watermark", "batch_seq", "inserted_records", "occurred_at", "schema_version"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("falta el campo JSON %q en %s", key, raw)
		}
	}

	var roundTrip contract.MeasurementsIngested
	if err := json.Unmarshal(raw, &roundTrip); err != nil || roundTrip != sampleEvent() {
		t.Fatalf("round-trip JSON falló: %v %+v", err, roundTrip)
	}
}
