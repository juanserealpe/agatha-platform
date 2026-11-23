package services

import (
	"context"
	"errors"
	"testing"
	"time"

	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var catchUpNow = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

// slots devuelve n timestamps cada 15 min (con los segundos del firmware) desde start.
func slots(start time.Time, n int) []time.Time {
	out := make([]time.Time, n)
	for i := range out {
		out[i] = start.Add(time.Duration(i)*15*time.Minute + 41*time.Second)
	}
	return out
}

type fakeCoverage struct {
	latest     map[int64]*time.Time
	timestamps map[int64][]time.Time
	listedFrom time.Time
	listedTo   time.Time
}

func (c *fakeCoverage) LatestTimestamp(_ context.Context, mp int64) (*time.Time, error) {
	return c.latest[mp], nil
}

func (c *fakeCoverage) ListTimestamps(_ context.Context, mp int64, from, to time.Time) ([]time.Time, error) {
	c.listedFrom, c.listedTo = from, to
	var out []time.Time
	for _, ts := range c.timestamps[mp] {
		if !ts.Before(from) && !ts.After(to) {
			out = append(out, ts)
		}
	}
	return out, nil
}

type ingestCall struct {
	mp         int64
	start, end time.Time
}

type fakeIngester struct {
	calls []ingestCall
	errs  map[int64]error
}

func (f *fakeIngester) IngestDataOs(_ context.Context, mp int64, start, end time.Time) error {
	f.calls = append(f.calls, ingestCall{mp, start, end})
	return f.errs[mp]
}

type fakeActiveDevices struct{ devices []entities.Device }

func (f fakeActiveDevices) FindByMeasuringPointID(context.Context, int64) (*entities.Device, error) {
	return nil, nil
}

func (f fakeActiveDevices) FindActive(context.Context) ([]entities.Device, error) {
	return f.devices, nil
}

var testCatchUpConfig = CatchUpConfig{
	Lookback:   24 * time.Hour,
	MinAge:     0,
	MaxStep:    20 * time.Minute,
	JoinWithin: 2 * time.Hour,
	Partition:  5 * 24 * time.Hour,
}

func newTestCatchUp(cfg CatchUpConfig, coverage *fakeCoverage, ingester *fakeIngester, mps ...int64) *CatchUpService {
	devices := make([]entities.Device, 0, len(mps))
	for _, mp := range mps {
		devices = append(devices, entities.Device{MeasuringPointID: mp, Active: true})
	}
	bogota, _ := time.LoadLocation("America/Bogota")
	return NewCatchUpService(fakeActiveDevices{devices}, coverage, ingester, cfg, bogota, logger.New("test", "Info"))
}

func TestCatchUp_CompleteCoverageRequestsNothing(t *testing.T) {
	ts := slots(catchUpNow.Add(-24*time.Hour), 96)
	coverage := &fakeCoverage{
		latest:     map[int64]*time.Time{11556: &ts[len(ts)-1]},
		timestamps: map[int64][]time.Time{11556: ts},
	}
	ingester := &fakeIngester{}

	results, err := newTestCatchUp(testCatchUpConfig, coverage, ingester, 11556).Run(context.Background(), catchUpNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(ingester.calls) != 0 {
		t.Fatalf("no debía pedir nada a DataOS: %+v", ingester.calls)
	}
	if len(results) != 1 || results[0].Gaps != 0 {
		t.Fatalf("resultado = %+v", results)
	}
}

func TestCatchUp_InternalHoleIsRequestedEvenWhenLatestIsRecent(t *testing.T) {
	// Caso VPS: los reintentos del callback ya trajeron lo reciente, pero
	// quedó un hueco adentro de la ventana.
	ts := slots(catchUpNow.Add(-24*time.Hour), 96)
	holeStart, holeEnd := ts[39], ts[48]
	withHole := append(append([]time.Time{}, ts[:40]...), ts[48:]...)
	coverage := &fakeCoverage{
		latest:     map[int64]*time.Time{11556: &ts[len(ts)-1]},
		timestamps: map[int64][]time.Time{11556: withHole},
	}
	ingester := &fakeIngester{}

	if _, err := newTestCatchUp(testCatchUpConfig, coverage, ingester, 11556).Run(context.Background(), catchUpNow); err != nil {
		t.Fatal(err)
	}
	if len(ingester.calls) != 1 {
		t.Fatalf("llamadas = %+v, se esperaba 1", ingester.calls)
	}
	got := ingester.calls[0]
	if !got.start.Equal(holeStart) || !got.end.Equal(holeEnd) {
		t.Fatalf("rango pedido = (%s, %s], se esperaba (%s, %s]", got.start, got.end, holeStart, holeEnd)
	}
	if got.start.Location().String() != "America/Bogota" {
		t.Fatalf("DataOS interpreta el rango en hora de Bogotá; llegó en %s", got.start.Location())
	}
}

func TestCatchUp_LongAbsenceStartsAtLatestNotAtLookback(t *testing.T) {
	// Caso local: la PC estuvo apagada 12 días, más que la ventana de 1 día.
	latest := catchUpNow.Add(-12 * 24 * time.Hour)
	coverage := &fakeCoverage{latest: map[int64]*time.Time{11556: &latest}}
	ingester := &fakeIngester{}

	if _, err := newTestCatchUp(testCatchUpConfig, coverage, ingester, 11556).Run(context.Background(), catchUpNow); err != nil {
		t.Fatal(err)
	}
	if !coverage.listedFrom.Equal(latest) {
		t.Fatalf("la búsqueda arrancó en %s, se esperaba el último dato %s", coverage.listedFrom, latest)
	}
	// 12 días en tramos de 5: 5 + 5 + 2.
	if len(ingester.calls) != 3 {
		t.Fatalf("llamadas = %d, se esperaban 3 tramos", len(ingester.calls))
	}
	if !ingester.calls[0].start.Equal(latest) || !ingester.calls[2].end.Equal(catchUpNow) {
		t.Fatalf("los tramos no cubren (%s, %s]: %+v", latest, catchUpNow, ingester.calls)
	}
	for i := 1; i < len(ingester.calls); i++ {
		if !ingester.calls[i].start.Equal(ingester.calls[i-1].end) {
			t.Fatalf("tramos con hueco o solape: %+v", ingester.calls)
		}
	}
}

func TestCatchUp_MinAgeLeavesRecentDataToTheCallback(t *testing.T) {
	// Caso VPS: el último registro tiene 40 min, pero lo reciente es del callback.
	latest := catchUpNow.Add(-40 * time.Minute)
	ts := slots(catchUpNow.Add(-24*time.Hour), 93)
	coverage := &fakeCoverage{
		latest:     map[int64]*time.Time{11556: &latest},
		timestamps: map[int64][]time.Time{11556: ts},
	}
	ingester := &fakeIngester{}
	cfg := testCatchUpConfig
	cfg.MinAge = time.Hour

	if _, err := newTestCatchUp(cfg, coverage, ingester, 11556).Run(context.Background(), catchUpNow); err != nil {
		t.Fatal(err)
	}
	if len(ingester.calls) != 0 {
		t.Fatalf("no debía pedir lo de la última hora: %+v", ingester.calls)
	}
	if !coverage.listedTo.Equal(catchUpNow.Add(-time.Hour)) {
		t.Fatalf("la búsqueda terminó en %s, se esperaba now-1h", coverage.listedTo)
	}
}

func TestCatchUp_NearbyHolesAreRequestedTogether(t *testing.T) {
	ts := slots(catchUpNow.Add(-24*time.Hour), 96)
	// Dos huecos separados por 1 h (< JoinWithin = 2 h) y uno lejano.
	var kept []time.Time
	for i, slot := range ts {
		if i == 10 || i == 15 || i == 80 {
			continue
		}
		kept = append(kept, slot)
	}
	coverage := &fakeCoverage{
		latest:     map[int64]*time.Time{11556: &ts[len(ts)-1]},
		timestamps: map[int64][]time.Time{11556: kept},
	}
	ingester := &fakeIngester{}

	if _, err := newTestCatchUp(testCatchUpConfig, coverage, ingester, 11556).Run(context.Background(), catchUpNow); err != nil {
		t.Fatal(err)
	}
	if len(ingester.calls) != 2 {
		t.Fatalf("llamadas = %+v, se esperaban 2 (huecos cercanos juntos)", ingester.calls)
	}
	if !ingester.calls[0].start.Equal(ts[9]) || !ingester.calls[0].end.Equal(ts[16]) {
		t.Fatalf("primer pedido = %+v", ingester.calls[0])
	}
}

func TestCatchUp_NoDataAndFailuresDoNotStopOtherDevices(t *testing.T) {
	coverage := &fakeCoverage{}
	ingester := &fakeIngester{errs: map[int64]error{
		11556: ingestionErrors.ErrRecivingIngestedData,
		11557: errors.New("DataOS caído"),
	}}

	results, err := newTestCatchUp(testCatchUpConfig, coverage, ingester, 11556, 11557, 11558).Run(context.Background(), catchUpNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || len(ingester.calls) != 3 {
		t.Fatalf("resultados = %+v, llamadas = %d", results, len(ingester.calls))
	}
	if results[0].NoData != 1 || results[0].Failed != 0 {
		t.Fatalf("11556 sin datos en DataOS no es una falla: %+v", results[0])
	}
	if results[1].Failed != 1 {
		t.Fatalf("11557 debía contar la falla: %+v", results[1])
	}
	if results[2].Requests != 1 || results[2].Failed != 0 {
		t.Fatalf("11558 debía pedirse igual: %+v", results[2])
	}
}
