package services

import (
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/services"
)

func TestChangeOf(t *testing.T) {
	updated := func(codes ...string) domainServices.Decision {
		d := domainServices.Decision{Outcome: domainServices.OutcomeUpdated}
		for _, c := range codes {
			d.Messages = append(d.Messages, entities.Message{Code: c})
		}
		return d
	}
	const (
		A = entities.LevelAmarillo
		O = entities.LevelNaranja
		R = entities.LevelRojo
	)

	tests := []struct {
		name            string
		decision        domainServices.Decision
		previous, after entities.Level
		want            string
	}{
		{"abre", domainServices.Decision{Outcome: domainServices.OutcomeOpened}, "", A, contract.ChangeOpened},
		{"se reactiva", domainServices.Decision{Outcome: domainServices.OutcomeReactivated}, "", A, contract.ChangeReactivated},
		{"finaliza", domainServices.Decision{Outcome: domainServices.OutcomeFinished}, A, A, contract.ChangeFinished},
		{"sube", updated(messages.CodeUpRojo), O, R, contract.ChangeLevelRaised},
		{"ruido que se une y sube", updated(messages.CodeLeakJoined), A, O, contract.ChangeLevelRaised},
		{"baja", updated(messages.CodeDowngrade), O, A, contract.ChangeLevelLowered},
		{"empieza sin datos", updated(messages.CodeNoCalcStart), O, O, contract.ChangeNoDataStarted},
		{"vuelven los datos", updated(messages.CodeNoCalcEnd), O, O, contract.ChangeNoDataEnded},
		{"vuelven los datos y sube: gana el nivel", updated(messages.CodeNoCalcEnd, messages.CodeUpRojo), O, R, contract.ChangeLevelRaised},
		{"seguimiento sin cambio visible", updated(messages.CodeTrend), O, O, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := changeOf(tt.decision, tt.previous, tt.after); got != tt.want {
				t.Fatalf("changeOf = %q, se esperaba %q", got, tt.want)
			}
		})
	}
}
