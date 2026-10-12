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

func newReplayService(h *fakeHistory) *ReplayService {
	return NewReplayService(h, &fakeEvents{}, domainServices.DefaultConfig(), logger.New("BaselineTest", "Error"))
}

// poisoned agrega datos extremos a partir de asOf: si la simulación los
// usara, las versiones diferirían de un Recalculate con el mismo asOf.
func poisoned(days int) []features.Sample {
	out := noise(days)
	for i := 0; i < 5*96; i++ {
		out = append(out, features.Sample{
			T:  asOf.Add(time.Duration(i) * features.Slot),
			B1: 5000, B2: 5000, Pl: 50, HasB1: true, HasB2: true, HasPl: true,
		})
	}
	return out
}

func TestSimulateVersions(t *testing.T) {
	const point = 7
	h := &fakeHistory{samples: map[int64][]features.Sample{point: poisoned(40)}, firstSeen: ptrTime(asOf.AddDate(0, 0, -40))}
	from := asOf.AddDate(0, 0, -30)

	versions, firstSeen, err := newReplayService(h).SimulateVersions(context.Background(), point, from, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) < 2 {
		t.Fatalf("versiones = %d, se esperaban varias", len(versions))
	}
	if firstSeen == nil || !firstSeen.Equal(*h.firstSeen) {
		t.Fatalf("first_seen = %v", firstSeen)
	}
	if h.samplesCall != 1 {
		t.Fatalf("lecturas = %d, se esperaba una sola lectura del histórico", h.samplesCall)
	}

	recalc := newService(h, &fakeBaselines{})

	for i, v := range versions {
		if v.ID != 0 || v.Version != i+1 {
			t.Fatalf("versión %d: id = %d, número = %d", i, v.ID, v.Version)
		}
		if v.ValidFrom.Before(from) || !v.ValidFrom.Before(asOf) {
			t.Fatalf("versión %d fuera del rango: %v", i, v.ValidFrom)
		}

		// Misma línea base que un recálculo real con ese asOf: solo usó datos anteriores.
		want, err := recalc.Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: v.ValidFrom, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v.Signal, want.Version.Signal) || !reflect.DeepEqual(v.Components, want.Version.Components) {
			t.Fatalf("versión %d (asOf %v) difiere de Recalculate: usó datos posteriores a asOf", i, v.ValidFrom)
		}

		if i == 0 {
			if v.Trigger != entities.TriggerBootstrap || !v.ValidFrom.Equal(from) {
				t.Fatalf("primera versión = %s en %v", v.Trigger, v.ValidFrom)
			}
			continue
		}

		prev := versions[i-1]
		if prev.ValidTo == nil || !prev.ValidTo.Equal(v.ValidFrom) {
			t.Fatalf("vigencia de la versión %d no se cierra en la siguiente", i-1)
		}

		weak := prev.Confidence == entities.ConfidenceLow || entities.HasFlag(prev.Flags, entities.FlagNoExclusionHistory)
		wantGap := 168 * time.Hour
		if weak {
			wantGap = 24 * time.Hour
		}
		if gap := v.ValidFrom.Sub(prev.ValidFrom); gap != wantGap {
			t.Fatalf("versión %d: separación %v, se esperaba %v (confianza previa %s)", i, gap, wantGap, prev.Confidence)
		}
	}

	if last := versions[len(versions)-1]; last.ValidTo != nil {
		t.Fatal("la última versión simulada queda vigente")
	}
}

func TestSimulateVersionsWithoutHistoryWaitsForData(t *testing.T) {
	const point = 7
	// Los datos empiezan 10 días después de from: hasta entonces no hay versión.
	h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(20)}}
	from := asOf.AddDate(0, 0, -30)

	versions, _, err := newReplayService(h).SimulateVersions(context.Background(), point, from, asOf)
	if err != nil || len(versions) == 0 {
		t.Fatalf("versiones = %d, err = %v", len(versions), err)
	}
	if first := versions[0].ValidFrom; first.Before(asOf.AddDate(0, 0, -20)) {
		t.Fatalf("primera versión en %v: antes de que existieran datos", first)
	}
}

func TestComputeAt(t *testing.T) {
	const point = 7
	h := &fakeHistory{samples: map[int64][]features.Sample{point: poisoned(30)}}

	got, err := newReplayService(h).ComputeAt(context.Background(), point, asOf)
	if err != nil || got.Version == nil {
		t.Fatalf("version = %v, err = %v", got.Version, err)
	}

	want, _ := newService(h, &fakeBaselines{}).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf, DryRun: true})
	if !reflect.DeepEqual(got.Version.Signal, want.Version.Signal) || got.Version.Version != 1 || !got.Version.ValidFrom.Equal(asOf) {
		t.Fatalf("ComputeAt = %+v", got.Version)
	}

	empty := &fakeHistory{samples: map[int64][]features.Sample{}}
	if got, err := newReplayService(empty).ComputeAt(context.Background(), point, asOf); err != nil || got.Version != nil {
		t.Fatalf("sin histórico: version = %v, err = %v", got.Version, err)
	}
}

func TestReplayErrors(t *testing.T) {
	s := newReplayService(&fakeHistory{samplesErr: map[int64]error{7: errors.New("boom")}})

	if _, _, err := s.SimulateVersions(context.Background(), 0, asOf, asOf.Add(time.Hour)); !errors.Is(err, domain.ErrInvalidMeasuringPointID) {
		t.Fatalf("punto inválido: %v", err)
	}
	if _, _, err := s.SimulateVersions(context.Background(), 7, asOf, asOf.Add(time.Hour)); !errors.Is(err, domain.ErrConsultingHistory) {
		t.Fatalf("falla de lectura: %v", err)
	}
	if _, err := s.ComputeAt(context.Background(), 7, asOf); !errors.Is(err, domain.ErrConsultingHistory) {
		t.Fatalf("falla de lectura: %v", err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
