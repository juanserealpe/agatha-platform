package services

import (
	"math"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

func b2Samples(values ...float64) []features.Sample {
	out := make([]features.Sample, len(values))
	for i, v := range values {
		out[i] = features.Sample{T: at(0).Add(time.Duration(i) * features.Slot), B2: v, HasB2: true}
	}
	return out
}

func TestSignalBaselineCalculator_BelowMinimumIsNotOK(t *testing.T) {
	calc := SignalBaselineCalculator{minRecords: 5}

	got := calc.Calculate(b2Samples(1, 2, 3, 4))

	if got.OK || got.Flags == nil || len(got.Flags) != 0 {
		t.Fatalf("con 4 de 5 registros se esperaba OK=false sin flags: %+v", got)
	}
}

func TestSignalBaselineCalculator_ComputesTheSignal(t *testing.T) {
	calc := SignalBaselineCalculator{minRecords: 5}
	samples := b2Samples(10, 20, 30, 40, 50, 60, 70, 80, 90, 100)
	samples[0].Pl, samples[0].HasPl = 4, true
	samples[1].Pl, samples[1].HasPl = 8, true
	samples[2].HasB2 = false // B2 ausente no cuenta

	got := calc.Calculate(samples)

	if !got.OK || len(got.Flags) != 0 {
		t.Fatalf("se esperaba OK sin flags: %+v", got)
	}
	sig := got.Signal
	wantB2 := []float64{10, 20, 40, 50, 60, 70, 80, 90, 100}
	if sig.NRecords != 9 || sig.NPlanitudRecords != 2 {
		t.Fatalf("conteos = %d B2 / %d Pl, se esperaba 9 / 2", sig.NRecords, sig.NPlanitudRecords)
	}
	if sig.MedianB2 != features.Median(wantB2) || sig.ThetaD != features.Quantile(wantB2, 0.9) {
		t.Fatalf("mediana=%v θD=%v", sig.MedianB2, sig.ThetaD)
	}
	if sig.DeltaMin != 10 {
		t.Fatalf("DeltaMin = %v, se esperaba 10", sig.DeltaMin)
	}
	if sig.P90Planitud == nil || *sig.P90Planitud != features.Quantile([]float64{4, 8}, 0.9) {
		t.Fatalf("P90 de Planitud mal calculado: %v", sig.P90Planitud)
	}
	if sig.MinElevatedB2 == nil || *sig.MinElevatedB2 <= sig.ThetaD {
		t.Fatalf("MinElevatedB2 debe ser el menor B2 por encima de θD: %v (θD %v)", sig.MinElevatedB2, sig.ThetaD)
	}
}

func TestSignalBaselineCalculator_ConstantSignalIsDegenerate(t *testing.T) {
	calc := SignalBaselineCalculator{minRecords: 3}

	got := calc.Calculate(b2Samples(50, 50, 50, 50))

	if !got.OK || got.Signal.MinElevatedB2 != nil || got.Signal.P90Planitud != nil {
		t.Fatalf("señal constante mal calculada: %+v", got)
	}
	if !entities.HasFlag(got.Flags, entities.FlagSignalDegenerate) || !entities.HasFlag(got.Flags, entities.FlagSignalLowVariability) {
		t.Fatalf("se esperaban SIGNAL_DEGENERATE y SIGNAL_LOW_VARIABILITY: %v", got.Flags)
	}
	if got.Signal.DeltaMin != defaultDeltaMin {
		t.Fatalf("sin diferencias positivas DeltaMin = %v, se esperaba %v", got.Signal.DeltaMin, defaultDeltaMin)
	}
}

func TestExclusionPolicy_MergesMarksIntoTwentyFourHourPeriods(t *testing.T) {
	policy := NewExclusionPolicy(DefaultConfig())
	from, to := at(0), at(24*30)

	marks := []HourMark{{T: at(100), I: 0.3}, {T: at(101), I: 0.5}, {T: at(200), I: 0.2}}
	got := policy.ApplyWithKnown(marks, nil, from, to)

	want := []entities.ExcludedPeriod{{From: at(76), To: at(101)}, {From: at(176), To: at(200)}}
	if len(got.Periods) != len(want) {
		t.Fatalf("periodos = %v, se esperaba %v", got.Periods, want)
	}
	for i := range want {
		if !got.Periods[i].From.Equal(want[i].From) || !got.Periods[i].To.Equal(want[i].To) {
			t.Fatalf("periodos = %v, se esperaba %v", got.Periods, want)
		}
	}
	if got.CapReached || len(got.MarkedHours) != 3 || !got.MarkedHours[0].Equal(at(100)) {
		t.Fatalf("marcas aceptadas mal ordenadas o tope alcanzado: %+v", got)
	}
}

func TestExclusionPolicy_CapKeepsTheHighestIndexFirst(t *testing.T) {
	policy := NewExclusionPolicy(DefaultConfig())
	from, to := at(0), at(72) // tope 40 % = 28,8 h: cabe una sola ventana de 24 h

	marks := []HourMark{{T: at(30), I: 0.2}, {T: at(70), I: 0.9}}
	got := policy.ApplyWithKnown(marks, nil, from, to)

	if !got.CapReached {
		t.Fatal("se esperaba CapReached")
	}
	if len(got.MarkedHours) != 1 || !got.MarkedHours[0].Equal(at(70)) {
		t.Fatalf("se debe aceptar primero la marca de mayor I: %v", got.MarkedHours)
	}
	if ratio := ExcludedRatio(got, from, to); math.Abs(ratio-24.0/72.0) > 1e-9 {
		t.Fatalf("ExcludedRatio = %v, se esperaba %v", ratio, 24.0/72.0)
	}
}

func TestExclusionPolicy_KnownEventsArePaddedClippedAndJoined(t *testing.T) {
	policy := NewExclusionPolicy(DefaultConfig()) // margen 2 h
	from, to := at(0), at(24*30)

	known := []entities.KnownEvent{
		{From: at(1), To: at(5)},     // el margen inferior se recorta al inicio del rango
		{From: at(300), To: at(310)}, // [298, 312)
		{From: at(-50), To: at(-10)}, // fuera del rango incluso con margen
	}
	marks := []HourMark{{T: at(320), I: 0.4}} // [296, 320) se une con [298, 312)

	got := policy.ApplyWithKnown(marks, known, from, to)

	want := []entities.ExcludedPeriod{{From: at(0), To: at(7)}, {From: at(296), To: at(320)}}
	if len(got.Periods) != len(want) {
		t.Fatalf("periodos = %v, se esperaba %v", got.Periods, want)
	}
	for i := range want {
		if !got.Periods[i].From.Equal(want[i].From) || !got.Periods[i].To.Equal(want[i].To) {
			t.Fatalf("periodos = %v, se esperaba %v", got.Periods, want)
		}
	}
	if len(got.MarkedHours) != 1 {
		t.Fatalf("los eventos conocidos no son marcas: %v", got.MarkedHours)
	}
}

func TestExclusionPolicy_KnownEventsCountTowardTheCap(t *testing.T) {
	policy := NewExclusionPolicy(DefaultConfig())
	from, to := at(0), at(100) // tope 40 h

	known := []entities.KnownEvent{{From: at(2), To: at(32)}} // 30 h + margen = 34 h
	got := policy.ApplyWithKnown([]HourMark{{T: at(90), I: 0.9}}, known, from, to)

	if !got.CapReached || len(got.MarkedHours) != 0 {
		t.Fatalf("34 h forzadas + 24 h de la marca superan el tope de 40 h: %+v", got)
	}
	if len(got.Periods) != 1 || !got.Periods[0].From.Equal(at(0)) || !got.Periods[0].To.Equal(at(34)) {
		t.Fatalf("el evento conocido se excluye aunque se alcance el tope: %v", got.Periods)
	}
}

func TestExcludedRatio_EmptyRangeIsZero(t *testing.T) {
	set := entities.ExclusionSet{Periods: []entities.ExcludedPeriod{{From: at(0), To: at(1)}}}
	if got := ExcludedRatio(set, at(5), at(5)); got != 0 {
		t.Fatalf("ExcludedRatio sobre un rango vacío = %v, se esperaba 0", got)
	}
}
