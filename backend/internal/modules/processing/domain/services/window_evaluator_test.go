package services

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

var firstSeen = h0.Add(-30 * 24 * time.Hour)

// window genera n samples cada 15 min que terminan (exclusivo) en end, con las
// tres series variando de forma determinista. Los primeros elevated tienen B2 alto.
func window(end time.Time, n, elevated int) []features.Sample {
	out := make([]features.Sample, n)
	start := end.Add(-time.Duration(n) * features.Slot)
	for i := range out {
		b2 := 50 + float64(i%7)
		if i >= n-elevated {
			b2 = 70 + float64(i%3)
		}
		out[i] = features.Sample{
			T:  start.Add(time.Duration(i) * features.Slot),
			B1: 10 + float64(i%5), B2: b2, Pl: 0.3 + float64(i%11)*0.01,
			HasB1: true, HasB2: true, HasPl: true,
		}
	}
	return out
}

func stat(median, mad float64) features.Stat { return features.Stat{Median: median, MAD: mad, N: 60} }

func snapshot() entities.BaselineSnapshot {
	fs := firstSeen
	return entities.BaselineSnapshot{
		Available:   true,
		VersionID:   7,
		Version:     3,
		ValidFrom:   h0.Add(-48 * time.Hour),
		Confidence:  entities.ConfidenceNormal,
		FirstSeenAt: &fs,
		Signal: features.Signal{
			MedianB2: 53, ThetaD: 56, P90Planitud: 0.38, HasP90Planitud: true,
			MinElevatedB2: 57, HasMinElevated: true, DeltaMin: 1,
		},
		Stats: map[int]features.ComponentStats{
			24: {D: stat(10, 3), P: stat(0.5, 0.25), Ex: stat(100, 40), C: stat(0.3, 0.05), HasC: true},
			6:  {D: stat(10, 4), P: stat(0.25, 0.25), Ex: stat(25, 10)},
		},
		Flags:            []string{},
		AlgorithmVersion: features.AlgorithmVersion,
	}
}

func evaluator() WindowEvaluator {
	return NewWindowEvaluator(EvaluatorConfig{ConfigVersion: "m=12,13,18", SilentPeriod: 7 * 24 * time.Hour})
}

func input(samples []features.Sample, snap entities.BaselineSnapshot) EvaluationInput {
	return EvaluationInput{
		PointID:   11556,
		At:        h0,
		Window24:  SliceWindow(samples, h0, 24),
		Window6:   SliceWindow(samples, h0, 6),
		Baseline:  snap,
		Trigger:   entities.TriggerEvent,
		CreatedAt: h0.Add(5 * time.Minute),
	}
}

func reason(e entities.FeatureEvaluation) string {
	return string(e.Status) + "/" + string(e.NoCalcReason)
}

func TestWindowEvaluatorStates(t *testing.T) {
	full := window(h0, 96, 8)

	none := snapshot()
	none.Available = false

	degenerate := snapshot()
	degenerate.Signal.HasMinElevated = false

	noStats24 := snapshot()
	noStats24.Stats = map[int]features.ComponentStats{24: {C: stat(0.3, 0.05), HasC: true}, 6: snapshot().Stats[6]}

	noStatC := snapshot()
	noStatC.Stats = map[int]features.ComponentStats{24: {D: stat(10, 3), P: stat(0.5, 0.25), Ex: stat(100, 40)}, 6: snapshot().Stats[6]}

	noB1 := window(h0, 96, 8)
	for i := range noB1 {
		noB1[i].HasB1 = false
	}

	lowCoverage := window(h0, 60, 8) // 62,5 % en 24 h; 6 h completa

	tests := []struct {
		name    string
		samples []features.Sample
		snap    entities.BaselineSnapshot
		want24  string
		want6   string
	}{
		{"sin línea base", full, none, "NO_CALCULABLE/NO_SIGNAL_BASELINE", "NO_CALCULABLE/NO_SIGNAL_BASELINE"},
		{"sin línea base precede a cobertura baja", lowCoverage, none, "NO_CALCULABLE/NO_SIGNAL_BASELINE", "NO_CALCULABLE/NO_SIGNAL_BASELINE"},
		{"señal degenerada", full, degenerate, "NO_CALCULABLE/SIGNAL_DEGENERATE", "NO_CALCULABLE/SIGNAL_DEGENERATE"},
		{"señal degenerada precede a cobertura baja", lowCoverage, degenerate, "NO_CALCULABLE/SIGNAL_DEGENERATE", "NO_CALCULABLE/SIGNAL_DEGENERATE"},
		{"cobertura baja solo en 24 h", lowCoverage, snapshot(), "NO_CALCULABLE/LOW_COVERAGE", "CALCULATED/"},
		{"cobertura baja precede a faltan estadísticas", lowCoverage, noStats24, "NO_CALCULABLE/LOW_COVERAGE", "CALCULATED/"},
		{"faltan estadísticas de 24 h", full, noStats24, "NO_CALCULABLE/NO_COMPONENT_BASELINE", "CALCULATED/"},
		{"falta la estadística de C afecta a ambas", full, noStatC, "NO_CALCULABLE/NO_COMPONENT_BASELINE", "NO_CALCULABLE/NO_COMPONENT_BASELINE"},
		{"faltan estadísticas precede a coherencia", noB1, noStats24, "NO_CALCULABLE/NO_COMPONENT_BASELINE", "NO_CALCULABLE/INSUFFICIENT_COHERENCE_DATA"},
		{"sin datos de coherencia", noB1, snapshot(), "NO_CALCULABLE/INSUFFICIENT_COHERENCE_DATA", "NO_CALCULABLE/INSUFFICIENT_COHERENCE_DATA"},
		{"calculada", full, snapshot(), "CALCULATED/", "CALCULATED/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e24, e6 := evaluator().Evaluate(input(tt.samples, tt.snap))

			if got := reason(e24); got != tt.want24 {
				t.Errorf("24 h = %s, se esperaba %s", got, tt.want24)
			}
			if got := reason(e6); got != tt.want6 {
				t.Errorf("6 h = %s, se esperaba %s", got, tt.want6)
			}
		})
	}
}

func TestWindowEvaluatorStoredValues(t *testing.T) {
	full := window(h0, 96, 8)

	t.Run("sin línea base guarda solo n y cobertura", func(t *testing.T) {
		snap := snapshot()
		snap.Available = false
		e24, _ := evaluator().Evaluate(input(full, snap))

		if e24.NExpected != 96 || e24.NReceived != 96 || e24.NValid != 96 || e24.Coverage != 100 {
			t.Fatalf("n/cobertura = %d/%d/%d/%g", e24.NExpected, e24.NReceived, e24.NValid, e24.Coverage)
		}
		if e24.D != nil || e24.PHours != nil || e24.Ex != nil || e24.C != nil || e24.DPl != nil || e24.I != nil {
			t.Fatal("no debe guardar componentes sin línea base")
		}
		if e24.BaselineVersionID != nil || e24.LowConfidence {
			t.Fatal("sin versión no hay baseline_version_id ni low_confidence")
		}
	})

	t.Run("sin estadísticas guarda componentes pero no scores", func(t *testing.T) {
		snap := snapshot()
		snap.Stats = map[int]features.ComponentStats{}
		e24, _ := evaluator().Evaluate(input(full, snap))

		if e24.D == nil || e24.PHours == nil || e24.Ex == nil || e24.C == nil || e24.DPl == nil {
			t.Fatal("debe guardar D, P, Ex, C y D_Pl")
		}
		if e24.ZD != nil || e24.SD != nil || e24.I != nil {
			t.Fatal("no debe guardar z, s ni I")
		}
	})

	t.Run("sin coherencia guarda D, P, Ex y D_Pl pero no C", func(t *testing.T) {
		noB1 := window(h0, 96, 8)
		for i := range noB1 {
			noB1[i].HasB1 = false
		}
		e24, e6 := evaluator().Evaluate(input(noB1, snapshot()))

		if e24.D == nil || e24.DPl == nil || e24.C != nil || e6.C != nil {
			t.Fatalf("D=%v DPl=%v C24=%v C6=%v", e24.D, e24.DPl, e24.C, e6.C)
		}
	})
}

func TestWindowEvaluatorMatchesKernel(t *testing.T) {
	samples := window(h0, 96, 8)
	snap := snapshot()
	e24, e6 := evaluator().Evaluate(input(samples, snap))

	c24 := features.ComputeComponents(SliceWindow(samples, h0, 24), 24, snap.Signal, true)
	sc24, ok := features.Score(c24, snap.Stats[24], 24, snap.Signal)
	if !ok {
		t.Fatal("el kernel no calcula la ventana de 24 h de referencia")
	}

	c6 := features.ComputeComponents(SliceWindow(samples, h0, 6), 6, snap.Signal, false)
	c6.C, c6.HasC = c24.C, c24.HasC
	st6 := snap.Stats[6]
	st6.C, st6.HasC = snap.Stats[24].C, true
	sc6, ok := features.Score(c6, st6, 6, snap.Signal)
	if !ok {
		t.Fatal("el kernel no calcula la ventana de 6 h de referencia")
	}

	assertClose(t, "I 24 h", e24.I, sc24.I)
	assertClose(t, "D 24 h", e24.D, c24.D)
	assertClose(t, "P 24 h", e24.PHours, c24.P)
	assertClose(t, "Ex 24 h", e24.Ex, c24.Ex)
	assertClose(t, "C 24 h", e24.C, c24.C)
	assertClose(t, "D_Pl 24 h", e24.DPl, c24.DPl)
	assertClose(t, "z_C 24 h", e24.ZC, sc24.ZC)
	assertClose(t, "s_P 24 h", e24.SP, sc24.SP)

	assertClose(t, "I 6 h", e6.I, sc6.I)
	assertClose(t, "C 6 h (de 24 h)", e6.C, c24.C)
	assertClose(t, "z_C 6 h (estadística de 24 h)", e6.ZC, sc6.ZC)
	assertClose(t, "z_D 6 h", e6.ZD, sc6.ZD)

	if e24.CFrom24h || !e6.CFrom24h {
		t.Fatalf("c_from_24h = %v/%v, se esperaba false/true", e24.CFrom24h, e6.CFrom24h)
	}
}

func TestWindowEvaluatorMetadata(t *testing.T) {
	samples := window(h0, 96, 8)
	seq := int64(42)
	in := input(samples, snapshot())
	in.BatchSeq = &seq

	e24, e6 := evaluator().Evaluate(in)

	for _, e := range []entities.FeatureEvaluation{e24, e6} {
		if e.MeasuringPointID != 11556 || !e.EvaluatedAt.Equal(h0) || !e.CreatedAt.Equal(in.CreatedAt) {
			t.Fatalf("identidad = %d %v %v", e.MeasuringPointID, e.EvaluatedAt, e.CreatedAt)
		}
		if *e.BaselineVersionID != 7 || *e.BaselineVersion != 3 || !e.BaselineValidFrom.Equal(snapshot().ValidFrom) {
			t.Fatalf("versión = %d/%d/%v", *e.BaselineVersionID, *e.BaselineVersion, e.BaselineValidFrom)
		}
		if e.AlgorithmVersion != features.AlgorithmVersion || e.ConfigVersion != "m=12,13,18" {
			t.Fatalf("versiones = %s/%s", e.AlgorithmVersion, e.ConfigVersion)
		}
		if e.Trigger != entities.TriggerEvent || *e.SourceBatchSeq != 42 {
			t.Fatalf("trigger/batch = %s/%v", e.Trigger, e.SourceBatchSeq)
		}
		if e.Flags == nil {
			t.Fatal("flags nunca es nil")
		}
	}

	if e24.WindowHours != 24 || !e24.WindowStart.Equal(h0.Add(-24*time.Hour)) || e24.NExpected != 96 {
		t.Fatalf("24 h = %d %v %d", e24.WindowHours, e24.WindowStart, e24.NExpected)
	}
	if e6.WindowHours != 6 || !e6.WindowStart.Equal(h0.Add(-6*time.Hour)) || e6.NExpected != 24 {
		t.Fatalf("6 h = %d %v %d", e6.WindowHours, e6.WindowStart, e6.NExpected)
	}
	if want := h0.Add(-features.Slot); !e24.InputMaxTs.Equal(want) || !e6.InputMaxTs.Equal(want) {
		t.Fatalf("input_max_ts = %v/%v, se esperaba %v", e24.InputMaxTs, e6.InputMaxTs, want)
	}

	empty := input(nil, snapshot())
	e24, _ = evaluator().Evaluate(empty)
	if e24.InputMaxTs != nil || e24.NoCalcReason != entities.ReasonLowCoverage {
		t.Fatalf("ventana vacía: input_max_ts=%v motivo=%s", e24.InputMaxTs, e24.NoCalcReason)
	}
}

func TestWindowEvaluatorFlags(t *testing.T) {
	t.Run("piso de MAD", func(t *testing.T) {
		snap := snapshot()
		st := snap.Stats[24]
		st.D.MAD, st.C.MAD = 0, 0
		snap.Stats[24] = st

		e24, e6 := evaluator().Evaluate(input(window(h0, 96, 8), snap))

		for _, f := range []string{"MAD_FLOOR_APPLIED_D", "MAD_FLOOR_APPLIED_C"} {
			if !slices.Contains(e24.Flags, f) {
				t.Errorf("24 h flags = %v, falta %s", e24.Flags, f)
			}
		}
		if !slices.Contains(e6.Flags, "MAD_FLOOR_APPLIED_C") {
			t.Errorf("6 h usa la estadística de C de 24 h: flags = %v", e6.Flags)
		}
	})

	t.Run("serie constante", func(t *testing.T) {
		samples := window(h0, 96, 8)
		for i := range samples {
			samples[i].B1 = 10
		}
		e24, e6 := evaluator().Evaluate(input(samples, snapshot()))

		if !slices.Contains(e24.Flags, "CONSTANT_SERIES_B1") || !slices.Contains(e6.Flags, "CONSTANT_SERIES_B1") {
			t.Fatalf("flags = %v / %v", e24.Flags, e6.Flags)
		}
	})

	t.Run("confianza baja", func(t *testing.T) {
		snap := snapshot()
		snap.Confidence = entities.ConfidenceLow
		e24, e6 := evaluator().Evaluate(input(window(h0, 96, 8), snap))

		if !e24.LowConfidence || !e6.LowConfidence || e24.Status != entities.StatusCalculated {
			t.Fatalf("low_confidence = %v/%v estado %s", e24.LowConfidence, e6.LowConfidence, e24.Status)
		}
	})
}

func TestWindowEvaluatorSilent(t *testing.T) {
	week := 7 * 24 * time.Hour

	tests := []struct {
		name      string
		firstSeen *time.Time
		want      bool
	}{
		{"justo en el borde de 7 días ya no es silencioso", ptr(h0.Add(-week)), false},
		{"un instante antes del borde es silencioso", ptr(h0.Add(-week + time.Nanosecond)), true},
		{"dispositivo antiguo", ptr(firstSeen), false},
		{"sin primer registro es silencioso", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := snapshot()
			snap.FirstSeenAt = tt.firstSeen
			e24, e6 := evaluator().Evaluate(input(window(h0, 96, 8), snap))

			if e24.Silent != tt.want || e6.Silent != tt.want {
				t.Fatalf("silent = %v/%v, se esperaba %v", e24.Silent, e6.Silent, tt.want)
			}
		})
	}
}

func assertClose(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, se esperaba %g", name, want)
		return
	}
	if math.Abs(*got-want) > 1e-12 {
		t.Errorf("%s = %g, se esperaba %g", name, *got, want)
	}
}
