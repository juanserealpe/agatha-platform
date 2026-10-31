// Package policies son las reglas de cada canal (Strategy): qué avisos salen
// por el push y cuáles por la web. Son funciones puras, sin infraestructura.
package policies

import "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"

// SkipReason es por qué un aviso no sale por un canal (para los logs).
type SkipReason string

const (
	SkipLate           SkipReason = "LATE"
	SkipFalseAlarm     SkipReason = "FALSE_ALARM"
	SkipLevelNotNotify SkipReason = "LEVEL_NOT_NOTIFIABLE"
	SkipUnknownCode    SkipReason = "UNKNOWN_CODE"
)

// Verdict es lo que decide una política sobre un aviso.
type Verdict struct {
	Send bool
	// Reason explica por qué no se envía (vacío si Send).
	Reason SkipReason
}

func send() Verdict                  { return Verdict{Send: true} }
func skip(reason SkipReason) Verdict { return Verdict{Reason: reason} }

// ChannelPolicy decide si un aviso sale por un canal.
type ChannelPolicy interface {
	Evaluate(n entities.AlertNotice) Verdict
}

// MobilePushPolicy es la regla del push (docs/CONTRATO_PUSH_FCM.md §4): solo
// alertas nuevas y subidas de nivel en Amarillo, Naranja o Rojo; nunca horas
// atrasadas ni falsas alertas.
type MobilePushPolicy struct{}

func (MobilePushPolicy) Evaluate(n entities.AlertNotice) Verdict {
	switch {
	case n.Late:
		return skip(SkipLate)
	case n.Classification == entities.ClassificationFalseAlarm:
		return skip(SkipFalseAlarm)
	}
	if _, ok := entities.PushLevelFor(n.Level); !ok {
		return skip(SkipLevelNotNotify)
	}
	if _, ok := entities.PushKindFor(n.Code); !ok {
		return skip(SkipUnknownCode)
	}
	return send()
}

// WebPolicy es la regla del canal web: todo aviso sale. Los atrasados también
// (con Late, para que el frontend los marque "Atrasado"), y las falsas
// alertas, porque el operador debe verlas en el panel.
type WebPolicy struct{}

func (WebPolicy) Evaluate(entities.AlertNotice) Verdict { return send() }
