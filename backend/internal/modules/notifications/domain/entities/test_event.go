package entities

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidTestEvent: el evento de prueba pedido no es válido.
var ErrInvalidTestEvent = errors.New("evento de prueba inválido")

// Cambios de una alerta, como los publica Alert Management en alert_changed.
const (
	ChangeOpened        = "OPENED"
	ChangeLevelRaised   = "LEVEL_RAISED"
	ChangeLevelLowered  = "LEVEL_LOWERED"
	ChangeReactivated   = "REACTIVATED"
	ChangeFinished      = "FINISHED"
	ChangeNoDataStarted = "NO_DATA_STARTED"
	ChangeNoDataEnded   = "NO_DATA_ENDED"
	ChangeClassified    = "CLASSIFIED"
	ChangeLabeled       = "LABELED"
)

// noticeChange es el cambio que acompaña a cada código de aviso.
var noticeChange = map[string]string{
	"OPEN":                 ChangeOpened,
	"NEW_AFTER_CLASSIFIED": ChangeOpened,
	"UP_NARANJA_PERSIST":   ChangeLevelRaised,
	"UP_NARANJA_FAST":      ChangeLevelRaised,
	"UP_ROJO":              ChangeLevelRaised,
	"REACTIVATED":          ChangeReactivated,
}

// silentChanges son los cambios que solo actualizan el mapa y el panel.
var silentChanges = map[string]bool{
	ChangeLevelLowered: true, ChangeFinished: true, ChangeNoDataStarted: true,
	ChangeNoDataEnded: true, ChangeClassified: true, ChangeLabeled: true,
}

// TestEvent es un evento de alerta inventado para probar los canales de punta a
// punta sin escribir alertas. Con Code sale un aviso (alert_notification) y su
// cambio (alert_changed); con Change, solo el cambio.
type TestEvent struct {
	MeasuringPointID int64
	// AlertID negativo: alerta inventada (nunca choca con una real); 0 = una
	// nueva. Positivo: una alerta real, solo para probar el aviso y que abra su
	// detalle (ver TargetsRealAlert).
	AlertID        int64
	EventType      string
	Level          string
	Code           string
	Change         string
	Classification string
	Late           bool
}

// Notifies dice si el evento incluye un aviso (banner, sonido, push).
func (e TestEvent) Notifies() bool { return e.Code != "" }

// TargetsRealAlert dice si el aviso apunta a una alerta real: se publica solo
// el aviso, nunca alert_changed, para no pisar su estado real en los clientes.
func (e TestEvent) TargetsRealAlert() bool { return e.AlertID > 0 }

// ChangeKind es el cambio que se publica en alert_changed.
func (e TestEvent) ChangeKind() string {
	if e.Notifies() {
		return noticeChange[e.Code]
	}
	return e.Change
}

// NewTestEvent valida y normaliza el pedido (mayúsculas; FUGA y OPEN por
// defecto). Code y Change son excluyentes.
func NewTestEvent(e TestEvent) (TestEvent, error) {
	e.EventType = strings.ToUpper(strings.TrimSpace(e.EventType))
	e.Level = strings.ToUpper(strings.TrimSpace(e.Level))
	e.Code = strings.ToUpper(strings.TrimSpace(e.Code))
	e.Change = strings.ToUpper(strings.TrimSpace(e.Change))
	e.Classification = strings.ToUpper(strings.TrimSpace(e.Classification))
	if e.EventType == "" {
		e.EventType = EventLeak
	}
	if e.Code == "" && e.Change == "" {
		e.Code = "OPEN"
	}

	invalid := func(format string, args ...any) (TestEvent, error) {
		return TestEvent{}, fmt.Errorf("%w: "+format, append([]any{ErrInvalidTestEvent}, args...)...)
	}
	switch {
	case e.MeasuringPointID <= 0:
		return invalid("measuring_point_id debe ser positivo")
	case e.AlertID > 0 && e.Change != "":
		return invalid("con una alerta real (alert_id positivo) solo se manda code: un change pisaría su estado real")
	case e.EventType != EventLeak && e.EventType != EventMovement:
		return invalid("event_type debe ser FUGA o MOVIMIENTO")
	case e.Level != LevelAmarillo && e.Level != LevelNaranja && e.Level != LevelRojo:
		return invalid("level debe ser AMARILLO, NARANJA o ROJO")
	case e.Code != "" && e.Change != "":
		return invalid("se envía code o change, no los dos")
	case e.Code != "" && noticeChange[e.Code] == "":
		return invalid("code no es un aviso del catálogo (OPEN, NEW_AFTER_CLASSIFIED, UP_NARANJA_PERSIST, UP_NARANJA_FAST, UP_ROJO, REACTIVATED)")
	case e.Change != "" && !silentChanges[e.Change]:
		return invalid("change debe ser LEVEL_LOWERED, FINISHED, NO_DATA_STARTED, NO_DATA_ENDED, CLASSIFIED o LABELED")
	case e.Classification != "" && e.Classification != ClassificationConfirmed && e.Classification != ClassificationFalseAlarm:
		return invalid("classification debe ser CONFIRMADA o FALSA_ALERTA")
	case (e.Change == ChangeClassified || e.Change == ChangeLabeled) && e.Classification == "":
		return invalid("%s necesita classification", e.Change)
	}
	return e, nil
}
