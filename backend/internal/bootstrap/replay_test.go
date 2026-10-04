package bootstrap

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline"
	processingServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/replay"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var replayFrom = time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

type replayReader struct {
	samples []features.Sample
	calls   int
}

func (r *replayReader) GetSamples(_ context.Context, _ int64, from, to time.Time) ([]features.Sample, error) {
	r.calls++
	out := []features.Sample{}
	for _, s := range r.samples {
		if !s.T.Before(from) && s.T.Before(to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *replayReader) GetLatestTimestamp(context.Context, int64) (*time.Time, error) {
	return nil, nil
}

func (r *replayReader) ListActiveMeasuringPoints(context.Context) ([]int64, error) {
	return nil, errors.New("el replay no lista puntos")
}

type fakeReplayer struct {
	versions []baseline.EffectiveBaseline
	fixed    baseline.EffectiveBaseline
	asOf     time.Time
}

func (f *fakeReplayer) SimulateVersions(context.Context, int64, time.Time, time.Time) ([]baseline.EffectiveBaseline, error) {
	return f.versions, nil
}

func (f *fakeReplayer) ComputeAt(_ context.Context, _ int64, asOf time.Time) (baseline.EffectiveBaseline, error) {
	f.asOf = asOf
	return f.fixed, nil
}

func replaySamples(from, to time.Time) []features.Sample {
	var out []features.Sample
	for i, t := 0, from; t.Before(to); i, t = i+1, t.Add(features.Slot) {
		out = append(out, features.Sample{
			T: t, B1: 10 + float64(i%5), B2: 50 + float64(i%7), Pl: 0.3 + float64(i%11)*0.01,
			HasB1: true, HasB2: true, HasPl: true,
		})
	}
	return out
}

func readCSV(t *testing.T, buf *bytes.Buffer) (rows []map[string]string) {
	t.Helper()
	records, err := csv.NewReader(buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records[1:] {
		row := map[string]string{}
		for i, h := range replay.Header {
			row[h] = r[i]
		}
		rows = append(rows, row)
	}
	return rows
}

func runTestReplay(t *testing.T, replayer *fakeReplayer, opts ReplayOptions) (ReplaySummary, []map[string]string, *replayReader) {
	t.Helper()
	reader := &replayReader{samples: replaySamples(replayFrom.Add(-48*time.Hour), replayFrom.Add(48*time.Hour))}
	var buf bytes.Buffer
	opts.Out = &buf
	opts.PointID = 11556

	summary, err := runReplay(context.Background(), reader, replayer, processingServices.DefaultConfig(), opts, logger.New("ReplayTest", "Error"))
	if err != nil {
		t.Fatal(err)
	}
	return summary, readCSV(t, &buf), reader
}

func TestReplayFixedUsesOneVersionForEveryHour(t *testing.T) {
	fixedAsOf := replayFrom.Add(30 * time.Hour)
	replayer := &fakeReplayer{fixed: effective(fixedAsOf)}

	summary, rows, reader := runTestReplay(t, replayer, ReplayOptions{
		From: replayFrom, To: replayFrom.Add(23 * time.Hour), Mode: ReplayModeFixed, FixedAsOf: fixedAsOf,
	})

	if summary.Hours != 24 || summary.Rows != 48 || len(rows) != 48 || summary.BaselineVersions != 1 {
		t.Fatalf("resumen = %+v, filas CSV = %d", summary, len(rows))
	}
	if reader.calls != 1 {
		t.Fatalf("lecturas de samples = %d, se esperaba una", reader.calls)
	}
	if !replayer.asOf.Equal(fixedAsOf) {
		t.Fatalf("ComputeAt con asOf %v", replayer.asOf)
	}
	if rows[0]["evaluated_at"] != "2026-07-28T00:00:00Z" || rows[47]["evaluated_at"] != "2026-07-28T23:00:00Z" {
		t.Fatalf("rango = %s … %s", rows[0]["evaluated_at"], rows[47]["evaluated_at"])
	}
	for _, r := range rows {
		if r["mode"] != "fixed" || r["trigger"] != "REPLAY" || r["baseline_version"] != "3" || r["status"] != "CALCULATED" {
			t.Fatalf("fila = %v", r)
		}
	}
}

func TestReplayFixedDefaultsAsOfToTo(t *testing.T) {
	replayer := &fakeReplayer{fixed: effective(replayFrom)}
	to := replayFrom.Add(5 * time.Hour)

	runTestReplay(t, replayer, ReplayOptions{From: replayFrom, To: to, Mode: ReplayModeFixed})

	if !replayer.asOf.Equal(to) {
		t.Fatalf("asOf = %v, se esperaba -to = %v", replayer.asOf, to)
	}
}

func TestReplayAsOfSwitchesVersionsAtValidFrom(t *testing.T) {
	v1 := effective(replayFrom.Add(2*time.Hour + 10*time.Minute))
	v1.Version = 1
	v2 := effective(replayFrom.Add(5 * time.Hour))
	v2.Version = 2

	summary, rows, _ := runTestReplay(t, &fakeReplayer{versions: []baseline.EffectiveBaseline{v1, v2}}, ReplayOptions{
		From: replayFrom, To: replayFrom.Add(6 * time.Hour), Mode: ReplayModeAsOf,
	})

	if summary.Hours != 7 || summary.BaselineVersions != 2 {
		t.Fatalf("resumen = %+v", summary)
	}

	want := map[string]string{
		"2026-07-28T00:00:00Z": "", // sin versión vigente
		"2026-07-28T02:00:00Z": "", // v1 vale desde 02:10
		"2026-07-28T03:00:00Z": "1",
		"2026-07-28T04:00:00Z": "1",
		"2026-07-28T05:00:00Z": "2", // ValidFrom inclusivo
		"2026-07-28T06:00:00Z": "2",
	}
	for _, r := range rows {
		if version, ok := want[r["evaluated_at"]]; ok && r["baseline_version"] != version {
			t.Fatalf("%s (%s h): versión %q, se esperaba %q", r["evaluated_at"], r["window_hours"], r["baseline_version"], version)
		}
		if r["baseline_version"] == "" && r["no_calc_reason"] != "NO_SIGNAL_BASELINE" {
			t.Fatalf("sin versión: motivo %q", r["no_calc_reason"])
		}
		if r["mode"] != "asof" {
			t.Fatalf("mode = %q", r["mode"])
		}
	}
}

func TestReplayOptionsValidation(t *testing.T) {
	var buf bytes.Buffer
	base := ReplayOptions{PointID: 1, From: replayFrom, To: replayFrom.Add(time.Hour), Mode: ReplayModeAsOf, Out: &buf}

	invalid := []func(o *ReplayOptions){
		func(o *ReplayOptions) { o.PointID = 0 },
		func(o *ReplayOptions) { o.To = o.From },
		func(o *ReplayOptions) { o.Mode = "live" },
		func(o *ReplayOptions) { o.Out = nil },
	}

	if err := base.validate(); err != nil {
		t.Fatalf("opciones válidas rechazadas: %v", err)
	}
	for i, mutate := range invalid {
		opts := base
		mutate(&opts)
		if opts.validate() == nil {
			t.Errorf("caso %d: se esperaba error", i)
		}
	}
}
