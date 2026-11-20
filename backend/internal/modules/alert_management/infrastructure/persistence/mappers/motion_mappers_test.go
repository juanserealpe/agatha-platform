package mappers

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

func TestToMotionNotice_ShiftAndReasons(t *testing.T) {
	started := time.Date(2026, 8, 25, 11, 15, 42, 0, time.UTC)
	p, r, rp, rr := -3.11, 25.56, -3.18, 25.42
	state := "TEMP_ESTABLE"
	n := ToMotionNotice(models.MotionEvaluationReadModel{
		ID: 9, MeasuringPointID: 11556, EvaluatedAt: time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC), Status: "CALCULATED",
		TempState: &state, TempMode: "MOVIMIENTO_Y_TEMPERATURA", ComputedLevel: "AMARILLO", InEpisode: true, EpisodeEvent: "NONE",
		Displaced: true, PitchNow: &p, RollNow: &r, RefPitch: &rp, RefRoll: &rr, ShiftStartedAt: &started, AlertsEnabled: true,
		Reasons: `[{"code":"TEMP_STABILIZED","data":{"temp_rate_max_6h":0.025,"temp_stable_from":"2026-08-25T11:15:42Z"}}]`,
	})
	if *n.ShiftPitch != 0.07 || *n.ShiftRoll != 0.14 || !n.ShiftStartedAt.Equal(started) || n.TempState != "TEMP_ESTABLE" {
		t.Fatalf("notice %+v", n)
	}
	reason := n.Reason(entities.MotionTempStabilized)
	if reason == nil || *reason.TempRateMax6h != 0.025 || !reason.TempStableFrom.Equal(started) {
		t.Fatalf("motivo %+v", reason)
	}
}
