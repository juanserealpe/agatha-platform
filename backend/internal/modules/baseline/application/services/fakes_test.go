package services

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var (
	errDB = errors.New("db caída")
	t0    = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
)

func at(hours int) time.Time { return t0.Add(time.Duration(hours) * time.Hour) }

func testLogger() *logger.Logger { return logger.New("BaselineTest", "Error") }

// testConfig usa 20 días de histórico para que los escenarios sean rápidos.
func testConfig() domainServices.Config {
	cfg := domainServices.DefaultConfig()
	cfg.HistoryDays = 20
	return cfg
}

// normalSamples genera una muestra cada 15 min con B2 ≈ 50 (45–55) y B1/Planitud correlacionadas.
func normalSamples(from, to time.Time) []features.Sample {
	out := []features.Sample{}
	i := 0
	for t := from; t.Before(to); t = t.Add(features.Slot) {
		b2 := 45 + float64((i*7)%11)
		out = append(out, features.Sample{
			T: t, B1: b2*0.8 + float64((i*3)%5), HasB1: true,
			B2: b2, HasB2: true, Pl: b2/10 + float64((i*5)%7)*0.1, HasPl: true,
		})
		i++
	}
	return out
}

type samplesCall struct{ from, to time.Time }

type fakeHistory struct {
	samples   []features.Sample
	err       error
	calls     []samplesCall
	firstSeen *time.Time
	firstErr  error
	points    []int64
	listErr   error
}

func (f *fakeHistory) GetSamples(_ context.Context, _ int64, from, to time.Time) ([]features.Sample, error) {
	f.calls = append(f.calls, samplesCall{from, to})
	if f.err != nil {
		return nil, f.err
	}
	out := []features.Sample{}
	for _, s := range f.samples {
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

type fakeEvents struct {
	events []entities.KnownEvent
	err    error
	calls  []samplesCall
}

func (f *fakeEvents) GetKnownEvents(_ context.Context, _ int64, from, to time.Time) ([]entities.KnownEvent, error) {
	f.calls = append(f.calls, samplesCall{from, to})
	return f.events, f.err
}

type fakeBaselines struct {
	latest    map[int64]*entities.BaselineVersion
	latestErr map[int64]error
	saveErr   error
	saved     []entities.BaselineVersion
	history   []entities.BaselineVersion
	historyEr error
}

func newFakeBaselines() *fakeBaselines {
	return &fakeBaselines{latest: map[int64]*entities.BaselineVersion{}, latestErr: map[int64]error{}}
}

func (f *fakeBaselines) GetLatest(_ context.Context, pointID int64) (*entities.BaselineVersion, error) {
	return f.latest[pointID], f.latestErr[pointID]
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

func (f *fakeBaselines) SaveHistory(_ context.Context, _ int64, versions []entities.BaselineVersion) ([]entities.BaselineVersion, error) {
	if f.historyEr != nil {
		return nil, f.historyEr
	}
	out := make([]entities.BaselineVersion, len(versions))
	for i, v := range versions {
		v.ID = int64(100 + i)
		out[i] = v
	}
	f.history = out
	return out, nil
}
