package mappers

import (
	"context"
	"encoding/csv"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type csvSource struct{ records []entities.RawRecord }

func (s csvSource) ListRecords(_ context.Context, _ int64, from, to time.Time) ([]entities.RawRecord, error) {
	i := sort.Search(len(s.records), func(k int) bool { return !s.records[k].Timestamp.Before(from) })
	j := sort.Search(len(s.records), func(k int) bool { return !s.records[k].Timestamp.Before(to) })
	return s.records[i:j], nil
}

func (s csvSource) FirstMeasurementAt(context.Context, int64) (*time.Time, error) {
	t := s.records[0].Timestamp
	return &t, nil
}

func (s csvSource) ListPoints(context.Context) ([]int64, error) {
	return []int64{entities.MotionPilotPointID}, nil
}

func loadInput(t *testing.T) []entities.RawRecord {
	t.Helper()
	return loadInputFile(t, "../../../domain/services/testdata/motion/motion_input.csv")
}

func loadInputFile(t *testing.T, path string) []entities.RawRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	opt := func(s string) *float64 {
		if s == "" {
			return nil
		}
		v, _ := strconv.ParseFloat(s, 64)
		return &v
	}
	var out []entities.RawRecord
	for _, r := range rows[1:] {
		ts, _ := time.Parse(time.RFC3339, r[0])
		out = append(out, entities.RawRecord{Timestamp: ts, Pitch: opt(r[1]), Roll: opt(r[2]), Deriva: opt(r[3]), Temp: opt(r[4])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}

// Las 3 182 horas reales del MP-11556 vuelven idénticas de motion_evaluations:
// la hora siguiente lee exactamente el estado que se guardó. El caso
// sintético creep agrega MOV_CUMULATIVE, MOV_LEVEL_UP y MOV_DAILY sin datos.
func TestMotionEvaluation_RoundTripOfTheWholeHistory(t *testing.T) {
	for _, path := range []string{"../../../domain/services/testdata/motion/motion_input.csv",
		"../../../domain/services/testdata/motion_v2/motion_input_creep.csv"} {
		t.Run(path[len(path)-20:], func(t *testing.T) { roundTripAll(t, loadInputFile(t, path)) })
	}
}

func roundTripAll(t *testing.T, records []entities.RawRecord) {
	now := time.Date(2026, 11, 17, 19, 30, 0, 0, time.UTC)
	clock := ports.ClockFunc(func() time.Time { return now })
	log := logger.New("MapperTest", "Error")
	thresholds := memory.NewMotionThresholdRepository(entities.MotionThresholdVersion{
		MeasuringPointID: entities.MotionPilotPointID, Version: 1, Params: entities.DefaultMotionParams(),
		ValidFrom: records[0].Timestamp, Reason: "r", CreatedBy: "c",
	})
	evaluations := memory.NewMotionEvaluationRepository()
	evaluator := services.NewMotionEvaluationService(thresholds, evaluations, nil, csvSource{records}, nil, clock, log)
	if _, err := services.NewMotionBackfillService(evaluator, services.MotionBackfillConfig{}).
		Backfill(context.Background(), services.MotionBackfillCommand{PointID: entities.MotionPilotPointID}); err != nil {
		t.Fatal(err)
	}

	rows := evaluations.All(entities.MotionPilotPointID)
	if len(rows) != 3182 {
		t.Fatalf("filas = %d", len(rows))
	}
	withReasons, withGaps := 0, 0
	codes := map[entities.ReasonCode]bool{}
	for _, want := range rows {
		for _, r := range want.Reasons {
			codes[r.Code] = true
		}
		got := ToMotionEvaluation(ToMotionEvaluationModel(want))
		if !reflect.DeepEqual(got, want) {
			for i := range want.Reasons {
				if i < len(got.Reasons) && !reflect.DeepEqual(got.Reasons[i], want.Reasons[i]) {
					t.Logf("motivo %d\n got  %+v\n want %+v", i, got.Reasons[i], want.Reasons[i])
				}
			}
			if !reflect.DeepEqual(got.State, want.State) {
				t.Logf("estado\n got  %+v\n want %+v", got.State, want.State)
			}
			if !reflect.DeepEqual(got.History, want.History) {
				t.Logf("historial distinto")
			}
			if !reflect.DeepEqual(got.RulesHit, want.RulesHit) {
				t.Logf("rules_hit\n got  %+v\n want %+v", got.RulesHit, want.RulesHit)
			}
			t.Fatalf("%v: la ida y vuelta cambió la fila", want.EvaluatedAt)
		}
		if len(want.Reasons) > 0 {
			withReasons++
		}
		if len(want.State.PendingGaps) > 0 {
			withGaps++
		}
	}
	t.Logf("códigos cubiertos: %d", len(codes))
	if withReasons == 0 || withGaps == 0 {
		t.Fatalf("el histórico no cubre motivos (%d) ni huecos pendientes (%d)", withReasons, withGaps)
	}
}

func TestMotionThresholdVersion_RoundTrip(t *testing.T) {
	to := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	v := entities.MotionThresholdVersion{
		ID: 7, MeasuringPointID: 11556, Version: 2, Params: entities.NewPointMotionParams(),
		ValidFrom: time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), ValidTo: &to,
		Reason: "r", CreatedBy: "c", CreatedAt: time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC),
	}
	if got := ToMotionThresholdVersion(ToMotionThresholdVersionModel(v)); !reflect.DeepEqual(got, v) {
		t.Fatalf("got %+v\nwant %+v", got, v)
	}
}

func TestMotionReasons_JSONHasTheCodeDataShape(t *testing.T) {
	started := time.Date(2026, 8, 25, 11, 15, 42, 0, time.UTC)
	dp, dr := 0.06, 0.115
	got := MotionReasonsJSON([]entities.MotionReason{
		{Code: entities.ReasonMovShift, DP: &dp, DR: &dr, StartedAt: &started},
		{Code: entities.ReasonTempVerifying},
	})
	want := `[{"code":"MOV_SHIFT","data":{"dp":0.06,"dr":0.115,"started_at":"2026-08-25T11:15:42Z"}},{"code":"TEMP_VERIFYING","data":{}}]`
	if got != want {
		t.Fatalf("reasons = %s\nse esperaba %s", got, want)
	}
	if MotionReasonsJSON(nil) != "[]" {
		t.Fatalf("sin motivos = %s", MotionReasonsJSON(nil))
	}
}
