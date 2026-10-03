package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const point int64 = 11556

func TestRecalculate(t *testing.T) {
	t.Run("crea y persiste una versión", func(t *testing.T) {
		h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(30)}}
		b := &fakeBaselines{}
		res, err := newService(h, b).Recalculate(context.Background(),
			RecalculateCommand{PointID: point, AsOf: asOf, Trigger: entities.TriggerBootstrap})
		if err != nil || !res.Created || len(b.saved) != 1 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		v := b.saved[0]
		if !v.ValidFrom.Equal(asOf) || !v.HistoryTo.Equal(asOf) || !v.HistoryFrom.Equal(asOf.AddDate(0, 0, -90)) {
			t.Fatalf("vigencia/histórico incorrectos: %+v", v)
		}
		if v.Trigger != entities.TriggerBootstrap || v.AlgorithmVersion == "" || !v.CreatedAt.Equal(asOf) {
			t.Fatalf("metadatos incorrectos: %+v", v)
		}
	})

	t.Run("dry-run no persiste y es reproducible", func(t *testing.T) {
		h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(30)}}
		b := &fakeBaselines{}
		svc := newService(h, b)
		cmd := RecalculateCommand{PointID: point, AsOf: asOf, Trigger: entities.TriggerManual, DryRun: true}

		first, err := svc.Recalculate(context.Background(), cmd)
		if err != nil || first.Created || first.SkippedReason != SkippedDryRun || len(b.saved) != 0 {
			t.Fatalf("res = %+v, err = %v", first, err)
		}
		if len(first.Passes) == 0 || first.Version.Signal.NRecords == 0 {
			t.Fatal("dry-run debe devolver la línea base calculada y las pasadas")
		}

		second, _ := svc.Recalculate(context.Background(), cmd)
		if !reflect.DeepEqual(first, second) {
			t.Fatal("dos dry-run con el mismo asOf difieren")
		}
	})

	t.Run("INSUFFICIENT_HISTORY no persiste ni falla", func(t *testing.T) {
		h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(1)[:50]}}
		b := &fakeBaselines{}
		res, err := newService(h, b).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf})
		if err != nil || res.Created || res.SkippedReason != SkippedInsufficientHistory || len(b.saved) != 0 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})

	t.Run("conflicto concurrente es no-op", func(t *testing.T) {
		h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(30)}}
		b := &fakeBaselines{saveErr: domain.ErrBaselineConcurrentUpdate}
		res, err := newService(h, b).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf})
		if err != nil || res.Created || res.SkippedReason != SkippedConcurrentUpdate {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})

	t.Run("error de lectura ⇒ ErrConsultingHistory sin error crudo", func(t *testing.T) {
		h := &fakeHistory{samplesErr: map[int64]error{point: errors.New("connection refused")}}
		_, err := newService(h, &fakeBaselines{}).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf})
		if err != domain.ErrConsultingHistory {
			t.Fatalf("err = %v, se esperaba ErrConsultingHistory", err)
		}
	})

	t.Run("error al guardar ⇒ ErrPersistingBaseline", func(t *testing.T) {
		h := &fakeHistory{samples: map[int64][]features.Sample{point: noise(30)}}
		b := &fakeBaselines{saveErr: errors.New("disk full")}
		_, err := newService(h, b).Recalculate(context.Background(), RecalculateCommand{PointID: point, AsOf: asOf})
		if err != domain.ErrPersistingBaseline {
			t.Fatalf("err = %v, se esperaba ErrPersistingBaseline", err)
		}
	})

	t.Run("punto inválido no consulta la BD", func(t *testing.T) {
		h := &fakeHistory{}
		_, err := newService(h, &fakeBaselines{}).Recalculate(context.Background(), RecalculateCommand{PointID: 0, AsOf: asOf})
		if err != domain.ErrInvalidMeasuringPointID || h.samplesCall != 0 {
			t.Fatalf("err = %v, llamadas = %d", err, h.samplesCall)
		}
	})
}

func TestRecalculateDue(t *testing.T) {
	t.Run("un punto que falla no detiene al resto", func(t *testing.T) {
		h := &fakeHistory{
			points:  []int64{1, 2, 3},
			samples: map[int64][]features.Sample{2: noise(30), 3: noise(30)},
		}
		b := &fakeBaselines{latestErr: map[int64]error{1: errors.New("timeout")}}

		summary, err := newService(h, b).RecalculateDue(context.Background(), asOf)
		if err != nil || len(summary) != 3 {
			t.Fatalf("summary = %+v, err = %v", summary, err)
		}
		if summary[0].Err != domain.ErrConsultingBaseline {
			t.Fatalf("punto 1: err = %v", summary[0].Err)
		}
		for _, item := range summary[1:] {
			if item.Err != nil || !item.Due || item.Trigger != entities.TriggerBootstrap || !item.Result.Created {
				t.Fatalf("punto %d: %+v", item.PointID, item)
			}
		}
	})

	t.Run("respeta la política: versión reciente no se recalcula", func(t *testing.T) {
		recent := &entities.BaselineVersion{CreatedAt: asOf.Add(-time.Hour), Confidence: entities.ConfidenceNormal}
		h := &fakeHistory{points: []int64{point}}
		b := &fakeBaselines{latest: map[int64]*entities.BaselineVersion{point: recent}}

		summary, err := newService(h, b).RecalculateDue(context.Background(), asOf)
		if err != nil || len(summary) != 1 || summary[0].Due || h.samplesCall != 0 {
			t.Fatalf("summary = %+v", summary)
		}
	})

	t.Run("avanzar el reloj 168 h dispara SCHEDULED", func(t *testing.T) {
		old := &entities.BaselineVersion{CreatedAt: asOf.Add(-168 * time.Hour), Confidence: entities.ConfidenceNormal}
		h := &fakeHistory{points: []int64{point}, samples: map[int64][]features.Sample{point: noise(30)}}
		b := &fakeBaselines{latest: map[int64]*entities.BaselineVersion{point: old}}

		summary, _ := newService(h, b).RecalculateDue(context.Background(), asOf)
		if !summary[0].Due || summary[0].Trigger != entities.TriggerScheduled {
			t.Fatalf("summary = %+v", summary[0])
		}
	})

	t.Run("fallo al listar puntos ⇒ ErrConsultingHistory", func(t *testing.T) {
		h := &fakeHistory{listErr: errors.New("boom")}
		if _, err := newService(h, &fakeBaselines{}).RecalculateDue(context.Background(), asOf); err != domain.ErrConsultingHistory {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestQueryService(t *testing.T) {
	log := logger.New("BaselineTest", "Info")
	first := asOf.AddDate(0, -3, 0)

	t.Run("sin versión ⇒ nil sin error y con first_seen_at", func(t *testing.T) {
		svc := NewQueryService(&fakeHistory{firstSeen: &first}, &fakeBaselines{}, log)
		res, err := svc.GetEffective(context.Background(), point, asOf)
		if err != nil || res.Version != nil || res.FirstSeenAt == nil || !res.FirstSeenAt.Equal(first) {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})

	t.Run("error de BD ⇒ ErrConsultingBaseline", func(t *testing.T) {
		svc := NewQueryService(&fakeHistory{}, &fakeBaselines{effErr: errors.New("boom")}, log)
		if _, err := svc.GetEffective(context.Background(), point, asOf); err != domain.ErrConsultingBaseline {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("punto inválido", func(t *testing.T) {
		svc := NewQueryService(&fakeHistory{}, &fakeBaselines{}, log)
		if _, err := svc.GetEffective(context.Background(), -1, asOf); err != domain.ErrInvalidMeasuringPointID {
			t.Fatalf("err = %v", err)
		}
	})
}
