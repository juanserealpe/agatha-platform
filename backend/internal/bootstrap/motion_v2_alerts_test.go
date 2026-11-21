package bootstrap

import (
	"context"
	"strings"
	"testing"
	"time"

	alertPorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	alertServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	alertEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	alertMemory "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
	ruleEngineEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// motion-v2 de punta a punta: MotionAssess (Rule Engine) → evento v2 →
// toMotionNotice → backfill de Alert Management (DecideMotion v8), sin FUGA.

const motionV2Dir = "../modules/rule_engine/domain/services/testdata/motion_v2/"

func runMotionOnly(t *testing.T, notices []alertEntities.MotionNotice) *alertMemory.AlertStore {
	t.Helper()
	log := logger.New("MotionV2AlertsTest", "Error")
	store := alertMemory.NewAlertStore()
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	processor := alertServices.NewProcessingService(store, alertMemory.NewPolicyStore(), noDevices{}, nil,
		alertPorts.ClockFunc(func() time.Time { return now }), log)
	replay := alertServices.NewReplayService(processor, sliceLevels(nil), store, alertServices.ReplayConfig{BatchSize: 500}, log).
		WithMotionSource(sliceMotions(notices))
	if _, err := replay.Backfill(context.Background(), alertServices.BackfillCommand{PointID: ruleEngineEntities.MotionPilotPointID}); err != nil {
		t.Fatal(err)
	}
	return store
}

// CA-7: con send_naranja = ACTIVE, rapido_grande abre en Amarillo, a las 17:00
// del 10/09 (22:00Z) emite MOV_LEVEL_UP a Naranja (Notificación) con un color
// nuevo y sigue en Naranja hasta el fin. {calculado} queda vacío.
func TestMotionV2_NaranjaActiveLevelUp(t *testing.T) {
	params := ruleEngineEntities.DefaultMotionParams()
	params.SendNaranja = ruleEngineEntities.MotionSendActive
	store := runMotionOnly(t, motionNoticesFrom(t, motionV2Dir+"motion_input_rapido_grande.csv", params))

	opened := time.Date(2026, 9, 10, 21, 0, 0, 0, time.UTC)
	var alert *alertEntities.Alert
	for _, a := range store.Alerts() {
		if a.OpenedAt.Equal(opened) {
			c := a
			alert = &c
		}
	}
	if alert == nil {
		t.Fatalf("no abrió la alerta del %v", opened)
	}
	if alert.MaxLevel != alertEntities.LevelNaranja || alert.CurrentLevel != alertEntities.LevelNaranja ||
		alert.MotionTableLevel != alertEntities.LevelNaranja || alert.State != alertEntities.AlertFinished {
		t.Fatalf("alerta %s: actual %s, máximo %s, calculado %s, %s", alert.Code, alert.CurrentLevel, alert.MaxLevel, alert.MotionTableLevel, alert.State)
	}
	segs := store.Segments(alert.ID)
	if len(segs) != 2 || segs[0].Level != alertEntities.LevelAmarillo || segs[1].Level != alertEntities.LevelNaranja ||
		!segs[1].StartedAt.Equal(time.Date(2026, 9, 10, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("colores %+v", segs)
	}
	var up, end *alertEntities.Message
	for _, m := range store.Messages() {
		if m.AlertID != alert.ID {
			continue
		}
		c := m
		switch m.Code {
		case "MOV_LEVEL_UP":
			up = &c
		case "MOV_LEVEL_OBSERVED":
			t.Fatalf("MOV_LEVEL_OBSERVED con Naranja activo: %q", m.Body)
		case "MOV_OPEN":
			if strings.Contains(m.Body, "correspondería") {
				t.Fatalf("MOV_OPEN con {calculado}: %q", m.Body)
			}
		case "END_NORMAL":
			end = &c
		}
	}
	if up == nil || up.Kind != alertEntities.KindNotification || !up.EvaluatedAt.Equal(segs[1].StartedAt) ||
		up.Title != "La alerta de movimiento subió a Naranja" || up.Body != "El roll ya cambió +0,12° en 3 horas. En este sensor, Naranja es desde 0,10° en 3 horas." {
		t.Fatalf("MOV_LEVEL_UP %+v", up)
	}
	if end == nil || !strings.Contains(end.Body, "Nivel máximo: Naranja.") || strings.Contains(end.Body, "correspondía") {
		t.Fatalf("END_NORMAL %+v", end)
	}
}

// Con la tabla de §5.3 (Naranja en observación) el mismo caso no sube de
// color: MOV_LEVEL_OBSERVED y "por su magnitud correspondía a Naranja" al final.
func TestMotionV2_NaranjaInObservation(t *testing.T) {
	store := runMotionOnly(t, motionNoticesFrom(t, motionV2Dir+"motion_input_rapido_grande.csv", ruleEngineEntities.DefaultMotionParams()))
	for _, a := range store.Alerts() {
		if !a.OpenedAt.Equal(time.Date(2026, 9, 10, 21, 0, 0, 0, time.UTC)) {
			continue
		}
		if a.MaxLevel != alertEntities.LevelAmarillo || a.MotionTableLevel != alertEntities.LevelNaranja || len(store.Segments(a.ID)) != 1 {
			t.Fatalf("alerta %s: máximo %s, calculado %s, colores %d", a.Code, a.MaxLevel, a.MotionTableLevel, len(store.Segments(a.ID)))
		}
		if a.MotionHistory == nil {
			t.Fatal("la alerta no guardó motion_history")
		}
		return
	}
	t.Fatal("no abrió la alerta del 10/09")
}
