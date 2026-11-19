package entities_test

import (
	"errors"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

func TestNewTestEvent_DefaultsToANewLeakAlert(t *testing.T) {
	got, err := entities.NewTestEvent(entities.TestEvent{MeasuringPointID: 11556, Level: " rojo "})
	if err != nil {
		t.Fatal(err)
	}
	want := entities.TestEvent{MeasuringPointID: 11556, EventType: entities.EventLeak, Level: entities.LevelRojo, Code: "OPEN"}
	if got != want {
		t.Fatalf("evento = %+v, se esperaba %+v", got, want)
	}
	if !got.Notifies() || got.ChangeKind() != entities.ChangeOpened {
		t.Fatalf("aviso = %v, cambio = %s", got.Notifies(), got.ChangeKind())
	}
}

func TestNewTestEvent_ChangeOnly(t *testing.T) {
	got, err := entities.NewTestEvent(entities.TestEvent{MeasuringPointID: 1, AlertID: -4, Level: "ROJO", Change: "finished"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Notifies() || got.ChangeKind() != entities.ChangeFinished {
		t.Fatalf("evento = %+v", got)
	}
}

func TestNewTestEvent_UpCodesRaiseTheLevel(t *testing.T) {
	got, err := entities.NewTestEvent(entities.TestEvent{MeasuringPointID: 1, Level: "NARANJA", Code: "UP_NARANJA_FAST"})
	if err != nil || got.ChangeKind() != entities.ChangeLevelRaised {
		t.Fatalf("evento = %+v, %v", got, err)
	}
}

// Con un id real solo se manda el aviso: sirve para probar que el clic abre el
// detalle de esa alerta, sin tocar su estado en el mapa ni en el panel.
func TestNewTestEvent_RealAlertOnlyNotifies(t *testing.T) {
	got, err := entities.NewTestEvent(entities.TestEvent{MeasuringPointID: 11556, AlertID: 23, Level: "ROJO", Code: "UP_ROJO"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.TargetsRealAlert() || !got.Notifies() {
		t.Fatalf("evento = %+v", got)
	}
	if fake, _ := entities.NewTestEvent(entities.TestEvent{MeasuringPointID: 1, AlertID: -1, Level: "ROJO"}); fake.TargetsRealAlert() {
		t.Fatal("un id negativo no es una alerta real")
	}
}

func TestNewTestEvent_RejectsInvalidRequests(t *testing.T) {
	cases := map[string]entities.TestEvent{
		"sin punto":                 {Level: "ROJO"},
		"cambio sobre alerta real":  {MeasuringPointID: 1, AlertID: 142, Level: "ROJO", Change: "FINISHED"},
		"evento desconocido":        {MeasuringPointID: 1, Level: "ROJO", EventType: "INCENDIO"},
		"nivel normal":              {MeasuringPointID: 1, Level: "NORMAL"},
		"code y change":             {MeasuringPointID: 1, Level: "ROJO", Code: "OPEN", Change: "FINISHED"},
		"code fuera del catálogo":   {MeasuringPointID: 1, Level: "ROJO", Code: "SUMMARY"},
		"change que avisa":          {MeasuringPointID: 1, Level: "ROJO", Change: "OPENED"},
		"clasificación inválida":    {MeasuringPointID: 1, Level: "ROJO", Classification: "QUIZAS"},
		"clasificar sin valor":      {MeasuringPointID: 1, Level: "ROJO", Change: "CLASSIFIED"},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := entities.NewTestEvent(ev); !errors.Is(err, entities.ErrInvalidTestEvent) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
