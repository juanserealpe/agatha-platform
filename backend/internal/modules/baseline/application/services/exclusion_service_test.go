package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

type fakeExclusions struct {
	upserted []entities.ExclusionPeriod
	err      error
	open     map[int64]int // filas ALERT abiertas por punto
	countErr error
	counted  []int64
}

func (f *fakeExclusions) CountOpenAlerts(_ context.Context, pointID int64) (int, error) {
	f.counted = append(f.counted, pointID)
	return f.open[pointID], f.countErr
}

func (f *fakeExclusions) UpsertAlert(_ context.Context, p entities.ExclusionPeriod) error {
	if f.err != nil {
		return f.err
	}
	f.upserted = append(f.upserted, p)
	return nil
}

type fakeAlertIntervals struct {
	periods []entities.ExclusionPeriod
	err     error
}

func (f *fakeAlertIntervals) ListAlertIntervals(context.Context) ([]entities.ExclusionPeriod, error) {
	return f.periods, f.err
}

func newExclusionService(repo *fakeExclusions, alerts *fakeAlertIntervals) *ExclusionService {
	return NewExclusionService(repo, alerts, testLogger())
}

func ptr(t time.Time) *time.Time { return &t }

func TestRecordAlert_UpsertsTheAlertPeriod(t *testing.T) {
	tests := []struct {
		name  string
		ended *time.Time
	}{
		{name: "alerta activa", ended: nil},
		{name: "alerta finalizada", ended: ptr(at(30))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeExclusions{}

			err := newExclusionService(repo, &fakeAlertIntervals{}).RecordAlert(context.Background(), AlertSnapshot{
				AlertID: 23, AlertCode: "ALR-000023", MeasuringPointID: 11556, StartedAt: at(10), EndedAt: tt.ended,
			})
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}

			if len(repo.upserted) != 1 {
				t.Fatalf("upserts = %d, se esperaba 1", len(repo.upserted))
			}
			got := repo.upserted[0]
			if got.Source != entities.ExclusionSourceAlert || got.SourceAlertCode != "ALR-000023" ||
				got.MeasuringPointID != 11556 || !got.StartedAt.Equal(at(10)) {
				t.Fatalf("período = %+v", got)
			}
			if (tt.ended == nil) != (got.EndedAt == nil) || (tt.ended != nil && !got.EndedAt.Equal(*tt.ended)) {
				t.Fatalf("ended_at = %v, se esperaba %v", got.EndedAt, tt.ended)
			}
		})
	}
}

func TestRecordAlert_StoresUTC(t *testing.T) {
	repo := &fakeExclusions{}
	cot := time.FixedZone("COT", -5*3600)
	started := time.Date(2026, 9, 1, 7, 0, 0, 0, cot)

	if err := newExclusionService(repo, &fakeAlertIntervals{}).RecordAlert(context.Background(), AlertSnapshot{
		AlertID: 1, AlertCode: "ALR-000001", MeasuringPointID: 1, StartedAt: started, EndedAt: ptr(started.Add(time.Hour)),
	}); err != nil {
		t.Fatal(err)
	}

	got := repo.upserted[0]
	if got.StartedAt.Location() != time.UTC || got.EndedAt.Location() != time.UTC {
		t.Fatalf("se esperaba UTC: %v %v", got.StartedAt, got.EndedAt)
	}
}

// Las alertas de prueba de Notifications (id <= 0, ALR-PRUEBA-…) no son reales.
func TestRecordAlert_IgnoresTestAlerts(t *testing.T) {
	repo := &fakeExclusions{}

	err := newExclusionService(repo, &fakeAlertIntervals{}).RecordAlert(context.Background(), AlertSnapshot{
		AlertID: -1, AlertCode: "ALR-PRUEBA-000001", MeasuringPointID: 11556, StartedAt: at(10),
	})
	if err != nil || len(repo.upserted) != 0 {
		t.Fatalf("err=%v upserts=%d, se esperaba ignorar", err, len(repo.upserted))
	}
}

func TestRecordAlert_RejectsInvalidSnapshots(t *testing.T) {
	valid := AlertSnapshot{AlertID: 1, AlertCode: "ALR-000001", MeasuringPointID: 1, StartedAt: at(10)}

	tests := []struct {
		name   string
		mutate func(*AlertSnapshot)
	}{
		{name: "sin código", mutate: func(a *AlertSnapshot) { a.AlertCode = "  " }},
		{name: "punto inválido", mutate: func(a *AlertSnapshot) { a.MeasuringPointID = 0 }},
		{name: "sin inicio", mutate: func(a *AlertSnapshot) { a.StartedAt = time.Time{} }},
		{name: "fin antes del inicio", mutate: func(a *AlertSnapshot) { a.EndedAt = ptr(at(9)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeExclusions{}
			snapshot := valid
			tt.mutate(&snapshot)

			err := newExclusionService(repo, &fakeAlertIntervals{}).RecordAlert(context.Background(), snapshot)
			if !errors.Is(err, domain.ErrInvalidAlertSnapshot) || len(repo.upserted) != 0 {
				t.Fatalf("err=%v upserts=%d", err, len(repo.upserted))
			}
		})
	}
}

func TestRecordAlert_PersistFailure(t *testing.T) {
	err := newExclusionService(&fakeExclusions{err: errDB}, &fakeAlertIntervals{}).RecordAlert(context.Background(),
		AlertSnapshot{AlertID: 1, AlertCode: "ALR-000001", MeasuringPointID: 1, StartedAt: at(10)})
	if !errors.Is(err, domain.ErrPersistingExclusions) {
		t.Fatalf("err=%v", err)
	}
}

func TestSeedFromAlerts_CopiesEveryAlert(t *testing.T) {
	repo := &fakeExclusions{}
	alerts := &fakeAlertIntervals{periods: []entities.ExclusionPeriod{
		{MeasuringPointID: 11556, StartedAt: at(10), EndedAt: ptr(at(20)), SourceAlertCode: "ALR-000001"},
		{MeasuringPointID: 11556, StartedAt: at(40), SourceAlertCode: "ALR-000002"},
		// inválida: sin código ⇒ se cuenta como omitida, no aborta
		{MeasuringPointID: 11556, StartedAt: at(50)},
	}}

	got, err := newExclusionService(repo, alerts).SeedFromAlerts(context.Background())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if got.Read != 3 || got.Upserted != 2 || got.Skipped != 1 || len(repo.upserted) != 2 {
		t.Fatalf("resultado = %+v upserts=%d", got, len(repo.upserted))
	}
	for _, p := range repo.upserted {
		if p.Source != entities.ExclusionSourceAlert {
			t.Fatalf("source = %q", p.Source)
		}
	}
}

func TestSeedFromAlerts_Errors(t *testing.T) {
	t.Run("falla la lectura de alerts", func(t *testing.T) {
		_, err := newExclusionService(&fakeExclusions{}, &fakeAlertIntervals{err: errDB}).SeedFromAlerts(context.Background())
		if !errors.Is(err, domain.ErrConsultingAlerts) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("falla el upsert", func(t *testing.T) {
		alerts := &fakeAlertIntervals{periods: []entities.ExclusionPeriod{
			{MeasuringPointID: 1, StartedAt: at(10), SourceAlertCode: "ALR-000001"},
		}}
		_, err := newExclusionService(&fakeExclusions{err: errDB}, alerts).SeedFromAlerts(context.Background())
		if !errors.Is(err, domain.ErrPersistingExclusions) {
			t.Fatalf("err=%v", err)
		}
	})
}
