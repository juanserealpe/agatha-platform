package bootstrap

import (
	"context"
	"sync"
	"testing"
	"time"

	alertContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/contract"
	notificationEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func alertNotification() alertContract.AlertNotification {
	return alertContract.AlertNotification{
		MessageID:        42,
		AlertID:          7,
		AlertCode:        "ALT-0007",
		MeasuringPointID: 11556,
		EventType:        "FUGA",
		EvaluatedAt:      time.Date(2026, 11, 3, 9, 0, 0, 0, time.FixedZone("COT", -5*3600)),
		Code:             "UP_ROJO",
		Level:            "ROJO",
		MaxLevel:         "ROJO",
		AlertState:       "ACTIVA",
		Title:            "Alerta roja",
		Body:             "El índice 0.42 superó el límite 0.30",
		Source:           alertContract.SourceCatchUp,
		Late:             true,
		SchemaVersion:    alertContract.SchemaVersion,
	}
}

func TestToAlertNotice_TranslatesTheContract(t *testing.T) {
	got := toAlertNotice(alertNotification())

	want := notificationEntities.AlertNotice{
		MessageID:        42,
		AlertID:          7,
		AlertCode:        "ALT-0007",
		MeasuringPointID: 11556,
		EventType:        "FUGA",
		EvaluatedAt:      time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC),
		Code:             "UP_ROJO",
		Level:            "ROJO",
		MaxLevel:         "ROJO",
		AlertState:       "ACTIVA",
		Title:            "Alerta roja",
		Body:             "El índice 0.42 superó el límite 0.30",
		Source:           "CATCH_UP",
		Late:             true,
	}
	if got != want {
		t.Fatalf("notice = %+v\nse esperaba %+v", got, want)
	}
	if got.EvaluatedAt.Location() != time.UTC {
		t.Fatalf("EvaluatedAt en %v, se esperaba UTC", got.EvaluatedAt.Location())
	}
}

type recordingNoticeHandler struct {
	mu      sync.Mutex
	notices []notificationEntities.AlertNotice
	done    chan struct{}
}

func (h *recordingNoticeHandler) Handle(_ context.Context, n notificationEntities.AlertNotice) {
	h.mu.Lock()
	h.notices = append(h.notices, n)
	h.mu.Unlock()
	close(h.done)
}

func TestSubscribeAlertNotification_DeliversTranslatedNotices(t *testing.T) {
	bus := eventbus.NewInMemory(nil)
	t.Cleanup(func() { _ = bus.Close(context.Background()) })

	handler := &recordingNoticeHandler{done: make(chan struct{})}
	unsubscribe := subscribeAlertNotification(bus, handler, logger.New("NotificationsTest", "Error"))
	t.Cleanup(unsubscribe)

	busAlertNotificationPublisher{bus: bus}.PublishAlertNotification(context.Background(), alertNotification())

	select {
	case <-handler.done:
	case <-time.After(2 * time.Second):
		t.Fatal("el aviso no llegó al módulo de notificaciones")
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.notices) != 1 || handler.notices[0].MessageID != 42 {
		t.Fatalf("notices = %+v", handler.notices)
	}
}
