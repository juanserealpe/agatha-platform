package services

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"

	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/services"
)

type fakeHistory struct {
	samples     map[int64][]features.Sample
	firstSeen   *time.Time
	points      []int64
	samplesErr  map[int64]error
	firstErr    error
	listErr     error
	samplesCall int
}

func (f *fakeHistory) GetSamples(_ context.Context, pointID int64, from, to time.Time) ([]features.Sample, error) {
	f.samplesCall++
	if err := f.samplesErr[pointID]; err != nil {
		return nil, err
	}
	out := []features.Sample{}
	for _, s := range f.samples[pointID] {
		if !s.T.Before(from) && s.T.Before(to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeHistory) GetFirstSeenAt(context.Context, int64) (*time.Time, error) {
	return f.firstSeen, f.firstErr
}

func (f *fakeHistory) ListActiveMeasuringPoints(context.Context) ([]int64, error) {
	return f.points, f.listErr
}

type fakeBaselines struct {
	latest    map[int64]*entities.BaselineVersion
	effective *entities.BaselineVersion
	saved     []entities.BaselineVersion
	saveErr   error
	latestErr map[int64]error
	effErr    error
}

func (f *fakeBaselines) GetEffective(context.Context, int64, time.Time) (*entities.BaselineVersion, error) {
	return f.effective, f.effErr
}

func (f *fakeBaselines) GetLatest(_ context.Context, pointID int64) (*entities.BaselineVersion, error) {
	if err := f.latestErr[pointID]; err != nil {
		return nil, err
	}
	return f.latest[pointID], nil
}

func (f *fakeBaselines) SaveNewVersion(_ context.Context, v entities.BaselineVersion) (entities.BaselineVersion, error) {
	if f.saveErr != nil {
		return entities.BaselineVersion{}, f.saveErr
	}
	v.ID = int64(len(f.saved) + 1)
	v.Version = len(f.saved) + 1
	f.saved = append(f.saved, v)
	return v, nil
}

var asOf = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

// noise genera days días de samples hasta asOf con ruido determinista.
func noise(days int) []features.Sample {
	rng := rand.New(rand.NewPCG(1, 2))
	start := asOf.AddDate(0, 0, -days)
	out := make([]features.Sample, 0, days*96)
	for i := 0; i < days*96; i++ {
		out = append(out, features.Sample{
			T:  start.Add(time.Duration(i) * features.Slot),
			B1: float64(rng.IntN(100)), B2: float64(40 + rng.IntN(31)), Pl: rng.Float64(),
			HasB1: true, HasB2: true, HasPl: true,
		})
	}
	return out
}

func newService(h *fakeHistory, b *fakeBaselines) *RecalculationService {
	return NewRecalculationService(h, b, domainServices.DefaultConfig(), logger.New("BaselineTest", "Info"),
		func() time.Time { return asOf })
}
