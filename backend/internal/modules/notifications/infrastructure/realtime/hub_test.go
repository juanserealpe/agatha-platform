package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func newTestHub(cfg HubConfig) *Hub {
	return NewHub(cfg, logger.New("RealtimeTest", "Error"))
}

func notice(messageID int64) entities.AlertNotice {
	return entities.AlertNotice{MessageID: messageID, AlertCode: "ALR-000142", Code: "OPEN", Level: entities.LevelRojo}
}

func connect(t *testing.T, h *Hub, lastEventID string) (*Client, []Frame) {
	t.Helper()
	c, pending, err := h.Connect(lastEventID)
	if err != nil {
		t.Fatalf("Connect(%q): %v", lastEventID, err)
	}
	return c, pending
}

func receive(t *testing.T, c *Client) Frame {
	t.Helper()
	select {
	case f, ok := <-c.Frames():
		if !ok {
			t.Fatal("el hub cerró el cliente")
		}
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó ningún frame")
	}
	return Frame{}
}

func publishNotices(t *testing.T, h *Hub, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if err := h.DeliverNotice(context.Background(), notice(id)); err != nil {
			t.Fatalf("DeliverNotice: %v", err)
		}
	}
}

func messageIDs(t *testing.T, frames []Frame) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(frames))
	for _, f := range frames {
		var p noticePayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			t.Fatalf("data inválido %q: %v", f.Data, err)
		}
		ids = append(ids, p.MessageID)
	}
	return ids
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHub_FansOutToEveryClient(t *testing.T) {
	h := newTestHub(HubConfig{})
	a, _ := connect(t, h, "")
	b, _ := connect(t, h, "")

	publishNotices(t, h, 7)

	for name, c := range map[string]*Client{"a": a, "b": b} {
		f := receive(t, c)
		if f.Event != EventAlertNotification || f.ID != h.epoch+"-1" {
			t.Fatalf("cliente %s recibió %+v", name, f)
		}
	}
}

func TestHub_DropsSlowClientWithoutBlockingOthers(t *testing.T) {
	h := newTestHub(HubConfig{ClientBuffer: 1})
	slow, _ := connect(t, h, "")
	fast, _ := connect(t, h, "")

	publishNotices(t, h, 1)
	receive(t, fast)
	publishNotices(t, h, 2) // slow tiene el buffer lleno: se desconecta
	receive(t, fast)

	<-slow.Frames() // el que alcanzó a entrar
	if _, ok := <-slow.Frames(); ok {
		t.Fatal("el cliente lento debería quedar cerrado")
	}
	if got := h.Clients(); got != 1 {
		t.Fatalf("clientes = %d, quiere 1", got)
	}
}

func TestHub_RejectsClientsAboveTheLimit(t *testing.T) {
	h := newTestHub(HubConfig{MaxClients: 1})
	first, _ := connect(t, h, "")

	if _, _, err := h.Connect(""); !errors.Is(err, ErrTooManyClients) {
		t.Fatalf("err = %v, quiere ErrTooManyClients", err)
	}

	h.Disconnect(first)
	h.Disconnect(first) // idempotente
	connect(t, h, "")
}

func TestHub_HeartbeatIsNotReplayed(t *testing.T) {
	h := newTestHub(HubConfig{HeartbeatInterval: 10 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	c, _ := connect(t, h, "")
	// Evento con nombre y sin id: el navegador lo entrega a JavaScript sin tocar su Last-Event-ID.
	if f := receive(t, c); string(f.Bytes()) != "event: ping\ndata: {}\n\n" {
		t.Fatalf("frame = %q, quiere el heartbeat", f.Bytes())
	}
	if _, pending := connect(t, h, h.epoch+"-0"); len(pending) != 0 {
		t.Fatalf("pendientes = %+v: el heartbeat no se guarda", pending)
	}
}

func TestHub_RunClosesClientsOnShutdown(t *testing.T) {
	h := newTestHub(HubConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()

	c, _ := connect(t, h, "")
	cancel()
	<-done
	if _, ok := <-c.Frames(); ok {
		t.Fatal("el cliente debería quedar cerrado al apagar el hub")
	}
}

func TestHub_ReplaysWhatTheClientMissed(t *testing.T) {
	h := newTestHub(HubConfig{})
	publishNotices(t, h, 10, 11, 12)

	_, pending := connect(t, h, h.epoch+"-1")
	if got := messageIDs(t, pending); !equalIDs(got, []int64{11, 12}) {
		t.Fatalf("reenviados = %v, quiere [11 12]", got)
	}

	_, upToDate := connect(t, h, h.epoch+"-3")
	if len(upToDate) != 0 {
		t.Fatalf("pendientes = %+v: el cliente estaba al día", upToDate)
	}
}

func TestHub_ReplayWrapsAroundTheBuffer(t *testing.T) {
	h := newTestHub(HubConfig{ReplaySize: 3})
	publishNotices(t, h, 1, 2, 3, 4, 5)

	_, pending := connect(t, h, h.epoch+"-2")
	if got := messageIDs(t, pending); !equalIDs(got, []int64{3, 4, 5}) {
		t.Fatalf("reenviados = %v, quiere [3 4 5]", got)
	}
}

func TestHub_ResyncWhenTheIDCannotBeReplayed(t *testing.T) {
	h := newTestHub(HubConfig{ReplaySize: 2})
	publishNotices(t, h, 1, 2, 3, 4)

	cases := map[string]string{
		"fuera del buffer":      h.epoch + "-1",
		"otro epoch (reinicio)": "123-4",
		"id del futuro":         h.epoch + "-9",
		"mal formado":           "basura",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			_, pending := connect(t, h, id)
			if len(pending) != 1 || pending[0].Event != EventResync {
				t.Fatalf("pendientes = %+v, quiere un resync", pending)
			}
		})
	}
}

func TestHub_ChangePayloadUsesNullForMissingValues(t *testing.T) {
	h := newTestHub(HubConfig{})
	c, _ := connect(t, h, "")

	err := h.DeliverChange(context.Background(), entities.AlertChange{
		AlertID: 5, AlertCode: "ALR-000005", Change: "FINISHED", State: "FINALIZADA", MaxLevel: entities.LevelNaranja,
	})
	if err != nil {
		t.Fatalf("DeliverChange: %v", err)
	}

	f := receive(t, c)
	if f.Event != EventAlertChanged {
		t.Fatalf("evento = %q", f.Event)
	}
	var got map[string]any
	if err := json.Unmarshal(f.Data, &got); err != nil {
		t.Fatalf("data inválido: %v", err)
	}
	for _, key := range []string{"current_level", "classification"} {
		if v, ok := got[key]; !ok || v != nil {
			t.Fatalf("%s = %v, quiere null", key, v)
		}
	}
	if got["alert_code"] != "ALR-000005" || got["change"] != "FINISHED" {
		t.Fatalf("data = %v", got)
	}
}

func TestFrame_Bytes(t *testing.T) {
	f := Frame{ID: "1-2", Event: EventAlertChanged, Data: []byte(`{"a":1}`)}
	if got, want := string(f.Bytes()), "id: 1-2\nevent: alert.changed\ndata: {\"a\":1}\n\n"; got != want {
		t.Fatalf("Bytes() = %q, quiere %q", got, want)
	}
	if got, want := string(resyncFrame.Bytes()), "event: resync\ndata: {}\n\n"; got != want {
		t.Fatalf("resync = %q, quiere %q", got, want)
	}
}
