package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	baselineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type recordingExclusions struct {
	snapshots []baselineServices.AlertSnapshot
	err       error
}

func (r *recordingExclusions) RecordAlert(_ context.Context, a baselineServices.AlertSnapshot) error {
	r.snapshots = append(r.snapshots, a)
	return r.err
}

// Con el publicador real de Alert Management y el bus real: cada foto llega a
// Baseline traducida a su propio tipo (Baseline no importa alert_management).
func TestBaselineExclusions_RecordsEveryAlertChangedThroughTheBus(t *testing.T) {
	log := logger.New("BaselineExclusionsTest", "Error")
	bus := eventbus.NewInMemory(nil)
	recorder := &recordingExclusions{}
	subscribeBaselineExclusions(bus, recorder, log)

	started := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	ended := started.Add(30 * time.Hour)
	publisher := busAlertChangePublisher{bus: bus}
	for _, c := range []alertContract.AlertChanged{
		{AlertID: 23, AlertCode: "ALR-000023", MeasuringPointID: 11556, Change: alertContract.ChangeOpened, StartedAt: started},
		{AlertID: 23, AlertCode: "ALR-000023", MeasuringPointID: 11556, Change: alertContract.ChangeFinished, StartedAt: started, EndedAt: &ended},
		{AlertID: 23, AlertCode: "ALR-000023", MeasuringPointID: 11556, Change: alertContract.ChangeClassified, Classification: "FALSA_ALERTA", StartedAt: started, EndedAt: &ended},
	} {
		if err := publisher.PublishAlertChanged(context.Background(), c); err != nil {
			t.Fatal(err)
		}
	}
	closeBus(t, bus)

	if len(recorder.snapshots) != 3 {
		t.Fatalf("Baseline recibió %d fotos, se esperaban 3 (todas, falsas alarmas incluidas)", len(recorder.snapshots))
	}
	open, finished := recorder.snapshots[0], recorder.snapshots[1]
	if open.AlertID != 23 || open.AlertCode != "ALR-000023" || open.MeasuringPointID != 11556 ||
		!open.StartedAt.Equal(started) || open.EndedAt != nil {
		t.Fatalf("foto del OPENED = %+v", open)
	}
	if finished.EndedAt == nil || !finished.EndedAt.Equal(ended) {
		t.Fatalf("foto del FINISHED = %+v", finished)
	}
}

// Un fallo al guardar se registra y no afecta a los demás eventos.
func TestBaselineExclusions_RecordFailureDoesNotStopTheSubscription(t *testing.T) {
	log := logger.New("BaselineExclusionsTest", "Error")
	bus := eventbus.NewInMemory(nil)
	recorder := &recordingExclusions{err: errors.New("db caída")}
	subscribeBaselineExclusions(bus, recorder, log)

	publisher := busAlertChangePublisher{bus: bus}
	for i := 0; i < 2; i++ {
		_ = publisher.PublishAlertChanged(context.Background(),
			alertContract.AlertChanged{AlertID: 1, AlertCode: "ALR-000001", MeasuringPointID: 1, StartedAt: time.Now()})
	}
	closeBus(t, bus)

	if len(recorder.snapshots) != 2 {
		t.Fatalf("recibió %d, se esperaban 2", len(recorder.snapshots))
	}
}
