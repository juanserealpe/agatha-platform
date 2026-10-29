package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
)

const clientID = "6f1c0e3a-1b2c-4d5e-8f90-1234567890ab"

func managementFixture(t *testing.T) (*fixture, *services.ManagementService, int64) {
	f := newFixture()
	f.process(t, entities.SourceLive, opening()...)
	svc := services.NewManagementService(f.store, ports.ClockFunc(func() time.Time { return f.now }), testLogger())
	return f, svc, f.store.Alerts()[0].ID
}

func web() services.ActionOrigin { return services.ActionOrigin{Origin: entities.OriginWeb} }

func mobile(id string) services.ActionOrigin {
	return services.ActionOrigin{Origin: entities.OriginMobile, DeviceIdentifier: "Celular cuadrilla 1", ClientRecordID: id}
}

func TestClassify_SavesWithObservationAndManagementMessage(t *testing.T) {
	f, svc, id := managementFixture(t)
	r, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationFalseAlarm,
		Observation: "Recorrido PK31-PK37 sin hallazgos", ActionOrigin: mobile(clientID),
	})
	if err != nil || r.Outcome != services.OutcomeSaved {
		t.Fatalf("resultado = %+v, %v", r, err)
	}
	a, _ := f.store.Alert(id)
	if a.Management.Status != entities.StatusClassifiedNoLabel || a.State != entities.AlertActive {
		t.Fatalf("alerta = %+v: clasificar no finaliza", a)
	}
	obs := f.store.Observations()
	if len(obs) != 1 || obs[0].Step != entities.StepClassification || obs[0].ClientRecordID != clientID {
		t.Fatalf("observaciones = %+v", obs)
	}
	last := f.store.Messages()[len(f.store.Messages())-1]
	if last.Code != messages.CodeClassified || last.Kind != entities.KindManagement || last.SegmentID == 0 {
		t.Fatalf("mensaje = %+v", last)
	}
}

// Escenario 16: prevalece la primera; la segunda queda como observación de conflicto.
func TestClassify_SecondIsSavedAsConflictObservation(t *testing.T) {
	f, svc, id := managementFixture(t)
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: web()}); err != nil {
		t.Fatal(err)
	}
	r, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationFalseAlarm, ActionOrigin: mobile(clientID),
	})
	if err != nil || r.Outcome != services.OutcomeConflict || r.Existing == nil || r.Existing.Classification != entities.ClassificationConfirmed {
		t.Fatalf("resultado = %+v, %v", r, err)
	}
	a, _ := f.store.Alert(id)
	if a.Management.Classification != entities.ClassificationConfirmed {
		t.Fatal("la primera clasificación no cambia")
	}
	obs := f.store.Observations()
	if len(obs) != 1 || obs[0].Kind != entities.ObservationClassificationConflict {
		t.Fatalf("observaciones = %+v", obs)
	}
}

// Escenario 18: reenviar el mismo client_record_id no duplica.
func TestClassify_SameClientRecordIsAlreadyReceived(t *testing.T) {
	f, svc, id := managementFixture(t)
	cmd := services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: mobile(clientID)}
	if _, err := svc.Classify(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	messagesBefore := len(f.store.Messages())
	r, err := svc.Classify(context.Background(), cmd)
	if err != nil || r.Outcome != services.OutcomeAlreadyReceived {
		t.Fatalf("resultado = %+v, %v", r, err)
	}
	if len(f.store.Messages()) != messagesBefore || len(f.store.Observations()) != 0 {
		t.Fatal("un reenvío no agrega nada")
	}
}

// Escenario 17: etiquetar sin clasificar, con la lista de la otra clasificación u OTRO sin texto.
func TestLabel_Validation(t *testing.T) {
	f, svc, id := managementFixture(t)
	if _, err := svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelLeakConfirmed, ActionOrigin: web()}); !errors.Is(err, domain.ErrNotClassified) {
		t.Fatalf("sin clasificar: err = %v", err)
	}
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationFalseAlarm, ActionOrigin: web()}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelLeakConfirmed, ActionOrigin: web()}); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Fatalf("otra lista: err = %v", err)
	}
	if _, err := svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelOther, ActionOrigin: web()}); !errors.Is(err, domain.ErrOtherTextRequired) {
		t.Fatalf("OTRO sin texto: err = %v", err)
	}
	if _, err := svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelHeavyTraffic, OtherText: "x", ActionOrigin: web()}); !errors.Is(err, domain.ErrOtherTextRequired) {
		t.Fatalf("texto con otra etiqueta: err = %v", err)
	}
	if a, _ := f.store.Alert(id); a.Management.Label != "" {
		t.Fatal("nada se guarda")
	}

	r, err := svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelOther, OtherText: "Prueba hidrostática", ActionOrigin: web()})
	if err != nil || r.Outcome != services.OutcomeSaved {
		t.Fatalf("resultado = %+v, %v", r, err)
	}
	a, _ := f.store.Alert(id)
	if a.Management.Status != entities.StatusClassifiedAndLabel || a.Management.LabelOtherText != "Prueba hidrostática" {
		t.Fatalf("gestión = %+v", a.Management)
	}

	r, err = svc.Label(context.Background(), services.LabelCommand{AlertID: id, Label: entities.LabelExtremeWeather, ActionOrigin: web()})
	if err != nil || r.Outcome != services.OutcomeConflict {
		t.Fatalf("segunda etiqueta = %+v, %v", r, err)
	}
}

func TestAddObservation(t *testing.T) {
	f, svc, id := managementFixture(t)
	if _, err := svc.AddObservation(context.Background(), services.ObservationCommand{AlertID: id, Text: "", ActionOrigin: web()}); !errors.Is(err, domain.ErrInvalidObservation) {
		t.Fatalf("vacía: err = %v", err)
	}
	cmd := services.ObservationCommand{AlertID: id, Text: "Se escucha agua", ActionOrigin: mobile(clientID)}
	if r, err := svc.AddObservation(context.Background(), cmd); err != nil || r.Outcome != services.OutcomeSaved {
		t.Fatalf("resultado = %+v, %v", r, err)
	}
	if r, err := svc.AddObservation(context.Background(), cmd); err != nil || r.Outcome != services.OutcomeAlreadyReceived {
		t.Fatalf("reenvío = %+v, %v", r, err)
	}
	if len(f.store.Observations()) != 1 {
		t.Fatalf("observaciones = %d", len(f.store.Observations()))
	}
}

func TestManagement_OriginAndAlertValidation(t *testing.T) {
	_, svc, id := managementFixture(t)
	noDevice := services.ActionOrigin{Origin: entities.OriginMobile}
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: noDevice}); !errors.Is(err, domain.ErrInvalidOrigin) {
		t.Fatalf("app sin device_identifier: err = %v", err)
	}
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: mobile("no-uuid")}); !errors.Is(err, domain.ErrInvalidClientRecordID) {
		t.Fatalf("client_record_id inválido: err = %v", err)
	}
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{AlertID: 999, Classification: entities.ClassificationConfirmed, ActionOrigin: web()}); !errors.Is(err, domain.ErrAlertNotFound) {
		t.Fatalf("inexistente: err = %v", err)
	}
}
