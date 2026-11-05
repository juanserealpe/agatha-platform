package policies_test

import (
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/policies"
)

func notice(code, level string) entities.AlertNotice {
	return entities.AlertNotice{MessageID: 1, AlertCode: "ALR-000142", Code: code, Level: level, EventType: entities.EventLeak}
}

func TestMobilePushPolicy(t *testing.T) {
	late := notice("UP_ROJO", entities.LevelRojo)
	late.Late = true
	falseAlarm := notice("UP_ROJO", entities.LevelRojo)
	falseAlarm.Classification = entities.ClassificationFalseAlarm
	confirmed := notice("UP_ROJO", entities.LevelRojo)
	confirmed.Classification = entities.ClassificationConfirmed

	tests := []struct {
		name   string
		notice entities.AlertNotice
		want   policies.Verdict
	}{
		{"alerta nueva", notice("OPEN", entities.LevelAmarillo), policies.Verdict{Send: true}},
		{"nueva tras clasificada", notice("NEW_AFTER_CLASSIFIED", entities.LevelNaranja), policies.Verdict{Send: true}},
		{"sube a rojo", notice("UP_ROJO", entities.LevelRojo), policies.Verdict{Send: true}},
		{"reactivación que supera el máximo", notice("REACTIVATED", entities.LevelNaranja), policies.Verdict{Send: true}},
		{"confirmada sigue notificando", confirmed, policies.Verdict{Send: true}},
		{"hora atrasada", late, policies.Verdict{Reason: policies.SkipLate}},
		{"falsa alerta", falseAlarm, policies.Verdict{Reason: policies.SkipFalseAlarm}},
		{"nivel normal", notice("OPEN", "NORMAL"), policies.Verdict{Reason: policies.SkipLevelNotNotify}},
		{"código desconocido", notice("TREND", entities.LevelNaranja), policies.Verdict{Reason: policies.SkipUnknownCode}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (policies.MobilePushPolicy{}).Evaluate(tt.notice); got != tt.want {
				t.Fatalf("Evaluate = %+v, se esperaba %+v", got, tt.want)
			}
		})
	}
}

func TestWebPolicySendsEverything(t *testing.T) {
	late := notice("UP_ROJO", entities.LevelRojo)
	late.Late = true
	falseAlarm := notice("OPEN", entities.LevelAmarillo)
	falseAlarm.Classification = entities.ClassificationFalseAlarm

	for _, n := range []entities.AlertNotice{notice("OPEN", entities.LevelAmarillo), late, falseAlarm} {
		if got := (policies.WebPolicy{}).Evaluate(n); !got.Send {
			t.Fatalf("Evaluate(%+v) = %+v: la web recibe todo", n, got)
		}
	}
}

// Las dos políticas cumplen la misma interfaz: los servicios no saben cuál usan.
var (
	_ policies.ChannelPolicy = policies.MobilePushPolicy{}
	_ policies.ChannelPolicy = policies.WebPolicy{}
)
