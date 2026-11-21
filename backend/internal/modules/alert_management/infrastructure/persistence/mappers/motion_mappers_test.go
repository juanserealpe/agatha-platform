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

// motion-v2: la fila trae table_level, episode_kind, history y rules_hit.
func TestToMotionNotice_V2Columns(t *testing.T) {
	tl, kind := "ROJO", "RAPIDO"
	hist := `{"until":"2026-09-15T14:00:00Z","kind":"RAPIDO","previous_episodes":[]}`
	hits := `[{"window_hours":1,"kind":"SHORT","level":"ROJO","deg":0.2,"axis":"pitch","delta":-0.25,"span_hours":1}]`
	n := ToMotionNotice(models.MotionEvaluationReadModel{
		ID: 1, MeasuringPointID: 11556, EvaluatedAt: time.Date(2026, 9, 15, 15, 0, 0, 0, time.UTC), Status: "CALCULATED",
		ComputedLevel: "AMARILLO", InEpisode: true, EpisodeEvent: "OPENED", AlertsEnabled: true,
		TableLevel: &tl, EpisodeKind: &kind, History: &hist, RulesHit: &hits,
		Reasons: `[{"code":"MOV_ABRUPT","data":{"axis":"pitch","delta":-0.25,"hours":1,"window_hours":1,"level":"ROJO","deg":0.2,"amarillo_deg":0.04}}]`,
	})
	r := n.Reason(entities.MotionAbrupt)
	if n.SchemaVersion != 2 || n.TableLevel != entities.LevelRojo || n.EpisodeKind != "RAPIDO" || n.History["kind"] != "RAPIDO" ||
		len(n.RulesHit) != 1 || r == nil || *r.Num("amarillo_deg") != 0.04 || r.Int("window_hours") != 1 {
		t.Fatalf("notice %+v", n)
	}
	if v1 := ToMotionNotice(models.MotionEvaluationReadModel{Status: "CALCULATED", Reasons: "[]"}); v1.SchemaVersion != 1 || v1.History != nil {
		t.Fatalf("fila v1 %+v", v1)
	}
}

// motion_history y motion_table_level vuelven iguales de alerts.
func TestAlert_MotionV2ColumnsRoundTrip(t *testing.T) {
	a := entities.Alert{ID: 5, EventType: entities.EventTypeMovement, State: entities.AlertActive, CurrentLevel: entities.LevelAmarillo,
		MaxLevel: entities.LevelAmarillo, MotionTableLevel: entities.LevelNaranja,
		MotionHistory: map[string]any{"kind": "ACUMULADO", "normal": []any{map[string]any{"window_hours": 168.0}}}}
	m := ToAlertModel(a)
	got := ToAlert(m)
	if got.MotionTableLevel != entities.LevelNaranja || got.MotionHistory["kind"] != "ACUMULADO" ||
		ProcessingColumns(a)["motion_table_level"] == nil || ProcessingColumns(a)["motion_history"] == nil {
		t.Fatalf("alerta %+v", got)
	}
	if fuga := ToAlertModel(entities.Alert{EventType: entities.EventTypeLeak}); fuga.MotionHistory != nil || fuga.MotionTableLevel != nil {
		t.Fatalf("FUGA con columnas de movimiento: %+v", fuga)
	}
}
