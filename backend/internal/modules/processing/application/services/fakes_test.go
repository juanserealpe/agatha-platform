package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var (
	errDB    = errors.New("connection refused")
	h0       = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	seenAt   = h0.Add(-60 * 24 * time.Hour)
	testLog  = logger.New("ProcessingTest", "Error")
	pointA   = int64(11556)
	pointB   = int64(11557)
	slotStep = features.Slot
)

// ---------------------------------------------------------------------------
// MeasurementReader

type fakeReader struct {
	mu          sync.Mutex
	samples     map[int64][]features.Sample
	points      []int64
	samplesErr  error
	latestErr   map[int64]error
	listErr     error
	samplesCall int
	latestCall  int
}

func (f *fakeReader) GetSamples(_ context.Context, pointID int64, from, to time.Time) ([]features.Sample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.samplesCall++
	if f.samplesErr != nil {
		return nil, f.samplesErr
	}
	out := []features.Sample{}
	for _, s := range f.samples[pointID] {
		if !s.T.Before(from) && s.T.Before(to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeReader) GetLatestTimestamp(_ context.Context, pointID int64) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.latestCall++
	if err := f.latestErr[pointID]; err != nil {
		return nil, err
	}
	var latest *time.Time
	for _, s := range f.samples[pointID] {
		if s.HasB2 && (latest == nil || s.T.After(*latest)) {
			t := s.T
			latest = &t
		}
	}
	return latest, nil
}

func (f *fakeReader) ListActiveMeasuringPoints(context.Context) ([]int64, error) {
	return f.points, f.listErr
}

// ---------------------------------------------------------------------------
// EvaluationRepository: aplica la unicidad (punto, ventana, hora) como la BD
// y cuenta los intentos de insertar una fila ya existente.

type evalKey struct {
	point  int64
	window int
	at     time.Time
}

type fakeEvaluations struct {
	mu          sync.Mutex
	rows        map[evalKey]entities.FeatureEvaluation
	conflicts   int
	saveCalls   int
	saveErr     error
	lastErr     error
	preexisting map[evalKey]bool // filas "insertadas por otro proceso" (conflicto sin estar en rows)
}

func newFakeEvaluations() *fakeEvaluations {
	return &fakeEvaluations{rows: map[evalKey]entities.FeatureEvaluation{}, preexisting: map[evalKey]bool{}}
}

func (f *fakeEvaluations) GetLastEvaluatedAt(_ context.Context, pointID int64) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastErr != nil {
		return nil, f.lastErr
	}
	var last *time.Time
	for k := range f.rows {
		if k.point == pointID && k.window == 24 && (last == nil || k.at.After(*last)) {
			at := k.at
			last = &at
		}
	}
	return last, nil
}

func (f *fakeEvaluations) SaveAll(_ context.Context, evals []entities.FeatureEvaluation) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls++
	if f.saveErr != nil {
		return 0, f.saveErr
	}
	inserted := 0
	for _, e := range evals {
		k := evalKey{e.MeasuringPointID, e.WindowHours, e.EvaluatedAt}
		if _, ok := f.rows[k]; ok || f.preexisting[k] {
			f.conflicts++
			continue
		}
		f.rows[k] = e
		inserted++
	}
	return inserted, nil
}

func (f *fakeEvaluations) count(pointID int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for k := range f.rows {
		if k.point == pointID {
			n++
		}
	}
	return n
}

func (f *fakeEvaluations) get(pointID int64, window int, at time.Time) (entities.FeatureEvaluation, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.rows[evalKey{pointID, window, at}]
	return e, ok
}

// assertContiguous falla si falta alguna hora (24 h y 6 h) en [from, to].
func (f *fakeEvaluations) missing(pointID int64, from, to time.Time) []string {
	var out []string
	for t := from; !t.After(to); t = t.Add(time.Hour) {
		for _, w := range []int{24, 6} {
			if _, ok := f.get(pointID, w, t); !ok {
				out = append(out, fmt.Sprintf("%dh@%s", w, t.Format(time.RFC3339)))
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// BaselineProvider

type fakeBaseline struct {
	mu     sync.Mutex
	snap   entities.BaselineSnapshot
	failAt *time.Time
	calls  int
}

func (f *fakeBaseline) GetEffective(_ context.Context, _ int64, at time.Time) (entities.BaselineSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failAt != nil && at.Equal(*f.failAt) {
		return entities.BaselineSnapshot{}, errDB
	}
	return f.snap, nil
}

func stat(median, mad float64) features.Stat { return features.Stat{Median: median, MAD: mad, N: 60} }

func availableBaseline() entities.BaselineSnapshot {
	fs := seenAt
	return entities.BaselineSnapshot{
		Available: true, VersionID: 1, Version: 1, ValidFrom: seenAt.Add(24 * time.Hour),
		Confidence: entities.ConfidenceNormal, FirstSeenAt: &fs,
		Signal: features.Signal{
			MedianB2: 53, ThetaD: 56, P90Planitud: 0.38, HasP90Planitud: true,
			MinElevatedB2: 57, HasMinElevated: true, DeltaMin: 1,
		},
		Stats: map[int]features.ComponentStats{
			24: {D: stat(10, 3), P: stat(0.5, 0.25), Ex: stat(100, 40), C: stat(0.3, 0.05), HasC: true},
			6:  {D: stat(10, 4), P: stat(0.25, 0.25), Ex: stat(25, 10)},
		},
		Flags: []string{}, AlgorithmVersion: features.AlgorithmVersion,
	}
}

// data genera samples cada 15 min en [from, to) con las tres series.
func data(from, to time.Time) []features.Sample {
	var out []features.Sample
	for i, t := 0, from; t.Before(to); i, t = i+1, t.Add(slotStep) {
		out = append(out, features.Sample{
			T: t, B1: 10 + float64(i%5), B2: 50 + float64(i%7), Pl: 0.3 + float64(i%11)*0.01,
			HasB1: true, HasB2: true, HasPl: true,
		})
	}
	return out
}

// ---------------------------------------------------------------------------

type fixture struct {
	reader   *fakeReader
	evals    *fakeEvaluations
	baseline *fakeBaseline
	now      time.Time
	service  *EvaluationService
}

func newFixture(now time.Time) *fixture {
	f := &fixture{
		reader:   &fakeReader{samples: map[int64][]features.Sample{}, latestErr: map[int64]error{}},
		evals:    newFakeEvaluations(),
		baseline: &fakeBaseline{snap: availableBaseline()},
		now:      now,
	}
	f.service = NewEvaluationService(
		f.reader, f.evals, f.baseline,
		ports.ClockFunc(func() time.Time { return f.now }),
		Config{
			MaxSlotsEvent:        24,
			MaxSlotsReconciler:   72,
			InitialLookbackHours: 72,
			Grace:                30 * time.Minute,
			ConfigVersion:        "m=12,13,18",
			SilentPeriod:         7 * 24 * time.Hour,
		},
		testLog,
	)
	return f
}
