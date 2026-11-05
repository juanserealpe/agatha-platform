package services_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

// managementChangesFixture es managementFixture con el publicador de cambios;
// los cambios del procesamiento horario se descartan.
func managementChangesFixture(t *testing.T) (*fixture, *services.ManagementService, *fakeChangePublisher, int64) {
	f, svc, id := managementFixture(t)
	changes := &fakeChangePublisher{}
	return f, svc.WithChangePublisher(changes), changes, id
}

// Clasificar un rojo publica la foto sin titileo: deja de titilar en el mapa.
func TestClassify_PublishesClassifiedSnapshot(t *testing.T) {
	f, svc, changes, id := managementChangesFixture(t)
	f.process(t, entities.SourceLive, notice(2, 0.40, entities.LevelRojo, entities.LevelRojo, entities.EpisodeNone, 3))
	if a, _ := f.store.Alert(id); !a.Blinking() {
		t.Fatalf("alerta = %+v: el rojo sin clasificar titila", a)
	}

	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationFalseAlarm, ActionOrigin: web(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := changes.changes(); !slices.Equal(got, []string{contract.ChangeClassified}) {
		t.Fatalf("cambios = %v", got)
	}
	c := changes.published[0]
	if c.AlertID != id || c.Blinking || c.CurrentLevel != "ROJO" || c.ManagementStatus != "CLASIFICADA_SIN_ETIQUETA" ||
		c.Classification != "FALSA_ALERTA" || !c.EvaluatedAt.Equal(h(2)) || !c.OccurredAt.Equal(f.now) {
		t.Fatalf("foto = %+v", c)
	}
}

func TestLabel_PublishesLabeledSnapshot(t *testing.T) {
	_, svc, changes, id := managementChangesFixture(t)
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationFalseAlarm, ActionOrigin: web(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Label(context.Background(), services.LabelCommand{
		AlertID: id, Label: entities.LabelExtremeWeather, ActionOrigin: web(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := changes.changes(); !slices.Equal(got, []string{contract.ChangeClassified, contract.ChangeLabeled}) {
		t.Fatalf("cambios = %v", got)
	}
	if c := changes.published[1]; c.ManagementStatus != "CLASIFICADA_CON_ETIQUETA" || c.Classification != "FALSA_ALERTA" {
		t.Fatalf("foto = %+v", c)
	}
}

// Un conflicto o un reenvío no cambian la alerta: no publican.
func TestManagement_ConflictAndResendPublishNothing(t *testing.T) {
	_, svc, changes, id := managementChangesFixture(t)
	cmd := services.ClassifyCommand{AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: mobile(clientID)}
	if _, err := svc.Classify(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if r, err := svc.Classify(context.Background(), cmd); err != nil || r.Outcome != services.OutcomeAlreadyReceived {
		t.Fatalf("reenvío = %+v, %v", r, err)
	}
	if r, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationFalseAlarm, ActionOrigin: web(),
	}); err != nil || r.Outcome != services.OutcomeConflict {
		t.Fatalf("conflicto = %+v, %v", r, err)
	}
	if got := changes.changes(); !slices.Equal(got, []string{contract.ChangeClassified}) {
		t.Fatalf("cambios = %v", got)
	}
}

// Ni un error de validación, ni una observación libre, ni una TX fallida publican.
func TestManagement_ErrorsAndObservationsPublishNothing(t *testing.T) {
	f, svc, changes, id := managementChangesFixture(t)
	if _, err := svc.Label(context.Background(), services.LabelCommand{
		AlertID: id, Label: entities.LabelExtremeWeather, ActionOrigin: web(),
	}); err == nil {
		t.Fatal("se esperaba error: la alerta no está clasificada")
	}
	if _, err := svc.AddObservation(context.Background(), services.ObservationCommand{
		AlertID: id, Text: "Recorrido sin hallazgos", ActionOrigin: web(),
	}); err != nil {
		t.Fatal(err)
	}
	f.store.FailNextTransaction(errors.New("bd caída"))
	if _, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: web(),
	}); err == nil {
		t.Fatal("se esperaba error")
	}
	if len(changes.published) != 0 {
		t.Fatalf("cambios = %v", changes.changes())
	}
}

// La foto se publica después del commit: si falla, la clasificación queda guardada.
func TestClassify_ChangePublishFailureKeepsTheManagement(t *testing.T) {
	f, svc, changes, id := managementChangesFixture(t)
	changes.err = errors.New("bus caído")
	r, err := svc.Classify(context.Background(), services.ClassifyCommand{
		AlertID: id, Classification: entities.ClassificationConfirmed, ActionOrigin: web(),
	})
	if err != nil || r.Outcome != services.OutcomeSaved {
		t.Fatalf("resultado = %+v, %v", r, err)
	}
	if a, _ := f.store.Alert(id); a.Management.Classification != entities.ClassificationConfirmed {
		t.Fatalf("alerta = %+v", a)
	}
}
