package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeEvents struct {
	events   []entities.KnownEvent
	err      error
	calls    int
	from, to time.Time
}

func (f *fakeEvents) GetKnownEvents(_ context.Context, _ int64, from, to time.Time) ([]entities.KnownEvent, error) {
	f.calls++
	f.from, f.to = from, to
	if f.err != nil {
		return nil, f.err
	}
	return f.events, nil
}

// knownEvent: 2 días de evento conocido, 10 días antes de asOf.
var knownEvent = entities.KnownEvent{From: asOf.AddDate(0, 0, -10), To: asOf.AddDate(0, 0, -8), Label: "fuga"}

func newServiceWithEvents(h *fakeHistory, b *fakeBaselines, e *fakeEvents) *RecalculationService {
	return NewRecalculationService(h, b, e, domainServices.DefaultConfig(), logger.New("BaselineTest", "Error"),
		func() time.Time { return asOf })
}

func TestRecalculateExcludesKnownEvents(t *testing.T) {
	const point = 7
	h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(40)}}
	b := &fakeBaselines{}
	e := &fakeEvents{events: []entities.KnownEvent{knownEvent}}

	result, err := newServiceWithEvents(h, b, e).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf})
	if err != nil || !result.Created {
		t.Fatalf("created = %v, err = %v", result.Created, err)
	}

	// Se consultan los eventos del mismo histórico que se lee.
	if e.calls != 1 || !e.from.Equal(asOf.AddDate(0, 0, -90)) || !e.to.Equal(asOf) {
		t.Fatalf("consultas = %d, rango = [%v, %v)", e.calls, e.from, e.to)
	}

	v := b.saved[0]
	if !entities.HasFlag(v.Flags, entities.FlagKnownEventsExcluded) {
		t.Fatalf("flags = %v, falta KNOWN_EVENTS_EXCLUDED", v.Flags)
	}

	covered := false
	for _, p := range v.ExcludedPeriods {
		if !p.From.After(knownEvent.From) && !p.To.Before(knownEvent.To) {
			covered = true
		}
	}
	if !covered {
		t.Fatalf("los periodos excluidos no cubren el evento conocido: %+v", v.ExcludedPeriods)
	}
	if v.Signal.NRecords > 40*96-2*96 {
		t.Fatalf("n = %d: la señal usó registros del evento conocido", v.Signal.NRecords)
	}
}

func TestRecalculateWithoutKnownEventsHasNoFlag(t *testing.T) {
	const point = 7
	h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(40)}}
	b := &fakeBaselines{}

	if _, err := newServiceWithEvents(h, b, &fakeEvents{}).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf}); err != nil {
		t.Fatal(err)
	}
	if entities.HasFlag(b.saved[0].Flags, entities.FlagKnownEventsExcluded) {
		t.Fatalf("flags = %v", b.saved[0].Flags)
	}
}

func TestRecalculateFailsWhenKnownEventsFail(t *testing.T) {
	const point = 7
	h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(40)}}
	b := &fakeBaselines{}

	// Sin la lista de eventos no se calcula: una línea base contaminada es peor que ninguna.
	result, err := newServiceWithEvents(h, b, &fakeEvents{err: errors.New("boom")}).
		Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf})

	if !errors.Is(err, domain.ErrConsultingKnownEvents) || result.Created || len(b.saved) != 0 {
		t.Fatalf("err = %v, created = %v, guardadas = %d", err, result.Created, len(b.saved))
	}
}

func TestReplayExcludesKnownEvents(t *testing.T) {
	const point = 7
	h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(40)}, firstSeen: ptrTime(asOf.AddDate(0, 0, -40))}
	e := &fakeEvents{events: []entities.KnownEvent{knownEvent}}
	log := logger.New("BaselineTest", "Error")
	replay := NewReplayService(h, e, domainServices.DefaultConfig(), log)

	t.Run("ComputeAt coincide con Recalculate", func(t *testing.T) {
		got, err := replay.ComputeAt(context.Background(), point, asOf)
		if err != nil || got.Version == nil {
			t.Fatalf("version = %v, err = %v", got.Version, err)
		}
		want, err := newServiceWithEvents(h, &fakeBaselines{}, e).
			Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Version.Signal, want.Version.Signal) || !reflect.DeepEqual(got.Version.Flags, want.Version.Flags) {
			t.Fatalf("replay = %+v %v, recálculo = %+v %v", got.Version.Signal, got.Version.Flags, want.Version.Signal, want.Version.Flags)
		}
		if !entities.HasFlag(got.Version.Flags, entities.FlagKnownEventsExcluded) {
			t.Fatalf("flags = %v", got.Version.Flags)
		}
	})

	t.Run("SimulateVersions marca solo las versiones cuyo histórico toca el evento", func(t *testing.T) {
		e.calls = 0
		versions, _, err := replay.SimulateVersions(context.Background(), point, asOf.AddDate(0, 0, -30), asOf)
		if err != nil || len(versions) < 2 {
			t.Fatalf("versiones = %d, err = %v", len(versions), err)
		}
		if e.calls != 1 {
			t.Fatalf("consultas de eventos = %d, se esperaba una sola", e.calls)
		}

		flagged := 0
		for i, v := range versions {
			has := entities.HasFlag(v.Flags, entities.FlagKnownEventsExcluded)
			if want := v.ValidFrom.After(knownEvent.From); has != want {
				t.Fatalf("versión %d (valid_from %v): flag = %v, se esperaba %v", i, v.ValidFrom, has, want)
			}
			if has {
				flagged++
			}
		}
		if flagged == 0 || flagged == len(versions) {
			t.Fatalf("versiones con flag = %d de %d: el caso no ejercita ambos lados", flagged, len(versions))
		}
	})

	t.Run("un fallo consultando eventos aborta", func(t *testing.T) {
		failing := NewReplayService(h, &fakeEvents{err: errors.New("boom")}, domainServices.DefaultConfig(), log)
		if _, _, err := failing.SimulateVersions(context.Background(), point, asOf.AddDate(0, 0, -30), asOf); !errors.Is(err, domain.ErrConsultingKnownEvents) {
			t.Fatalf("err = %v", err)
		}
		if _, err := failing.ComputeAt(context.Background(), point, asOf); !errors.Is(err, domain.ErrConsultingKnownEvents) {
			t.Fatalf("err = %v", err)
		}
	})
}
