package services

import (
	"context"
	"errors"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var errDB = errors.New("db caída")

func testLogger() *logger.Logger { return logger.New("DashboardTest", "Error") }

type fakeDevices struct {
	summaries []entities.DeviceSummary
	listErr   error

	exists     bool
	existsErr  error
	existsCall []int64
}

func (f *fakeDevices) GetAllDeviceSummaries(context.Context) ([]entities.DeviceSummary, error) {
	return f.summaries, f.listErr
}

func (f *fakeDevices) ExistsDevice(_ context.Context, id int64) (bool, error) {
	f.existsCall = append(f.existsCall, id)
	return f.exists, f.existsErr
}

type fakeMeasurements struct {
	rows  []entities.RawMeasurement
	err   error
	calls int
}

func (f *fakeMeasurements) GetMeasurements(context.Context, int64, time.Time, time.Time) ([]entities.RawMeasurement, error) {
	f.calls++
	return f.rows, f.err
}

type fakeSeriesReader struct {
	points map[string][]entities.SeriesPoint
	err    error
	asked  [][]string
}

func (f *fakeSeriesReader) Read(_ context.Context, _ int64, defs []entities.SeriesDefinition, _, _ time.Time,
) (map[string][]entities.SeriesPoint, error) {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = d.Key
	}
	f.asked = append(f.asked, keys)
	return f.points, f.err
}

// pastRange devuelve un rango válido que termina una hora antes de ahora.
func pastRange() (time.Time, time.Time) {
	end := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	return end.Add(-24 * time.Hour), end
}
