package realtime

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// newStreamServer levanta el handler en un servidor real; timeout se usa como
// ReadTimeout (el de cmd/platform) y WriteTimeout.
func newStreamServer(t *testing.T, hub *Hub, timeout time.Duration) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	NewStreamHandler(hub, logger.New("RealtimeTest", "Error")).RegisterRoutes(mux)
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.ReadTimeout = timeout
	srv.Config.WriteTimeout = timeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// sseStream lee los bloques de un text/event-stream (separados por línea vacía).
type sseStream struct {
	resp   *http.Response
	blocks chan string
}

func openStream(t *testing.T, srv *httptest.Server, query string, header http.Header) *sseStream {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/notifications/stream"+query, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })

	s := &sseStream{resp: resp, blocks: make(chan string, 16)}
	go func() {
		defer close(s.blocks)
		scanner := bufio.NewScanner(resp.Body)
		var block []string
		for scanner.Scan() {
			if line := scanner.Text(); line != "" {
				block = append(block, line)
				continue
			}
			s.blocks <- strings.Join(block, "\n")
			block = nil
		}
	}()
	return s
}

func (s *sseStream) next(t *testing.T) string {
	t.Helper()
	select {
	case b, ok := <-s.blocks:
		if !ok {
			t.Fatal("el stream se cerró")
		}
		return b
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó ningún bloque")
	}
	return ""
}

func (s *sseStream) closed(t *testing.T) {
	t.Helper()
	for {
		select {
		case _, ok := <-s.blocks:
			if !ok {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("el stream sigue abierto")
		}
	}
}

func waitClients(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for hub.Clients() != want {
		if time.Now().After(deadline) {
			t.Fatalf("clientes = %d, quiere %d", hub.Clients(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStreamHandler_SendsHeadersRetryAndEvents(t *testing.T) {
	hub := newTestHub(HubConfig{})
	s := openStream(t, newStreamServer(t, hub, 0), "", nil)

	if s.resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", s.resp.StatusCode)
	}
	for k, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := s.resp.Header.Get(k); got != want {
			t.Fatalf("%s = %q, quiere %q", k, got, want)
		}
	}
	if got := s.next(t); got != "retry: 3000" {
		t.Fatalf("primer bloque = %q", got)
	}

	waitClients(t, hub, 1)
	publishNotices(t, hub, 7)
	got := s.next(t)
	if !strings.HasPrefix(got, "id: "+hub.epoch+"-1\nevent: alert.notification\ndata: {") ||
		!strings.Contains(got, `"message_id":7`) {
		t.Fatalf("evento = %q", got)
	}
}

func TestStreamHandler_ReplaysFromHeaderOrQuery(t *testing.T) {
	hub := newTestHub(HubConfig{})
	srv := newStreamServer(t, hub, 0)
	publishNotices(t, hub, 10, 11)

	cases := map[string]struct {
		query  string
		header http.Header
	}{
		"cabecera Last-Event-ID":  {header: http.Header{"Last-Event-Id": {hub.epoch + "-1"}}},
		"parámetro last_event_id": {query: "?last_event_id=" + hub.epoch + "-1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := openStream(t, srv, tc.query, tc.header)
			s.next(t) // retry
			if got := s.next(t); !strings.HasPrefix(got, "id: "+hub.epoch+"-2\n") {
				t.Fatalf("reenviado = %q, quiere el evento 2", got)
			}
		})
	}
}

func TestStreamHandler_SendsResyncForAnUnknownID(t *testing.T) {
	hub := newTestHub(HubConfig{})
	s := openStream(t, newStreamServer(t, hub, 0), "?last_event_id=0-5", nil)
	s.next(t) // retry
	if got := s.next(t); got != "event: resync\ndata: {}" {
		t.Fatalf("bloque = %q, quiere resync", got)
	}
}

func TestStreamHandler_Rejects503AboveTheLimit(t *testing.T) {
	hub := newTestHub(HubConfig{MaxClients: 1})
	srv := newStreamServer(t, hub, 0)
	openStream(t, srv, "", nil).next(t)

	resp, err := srv.Client().Get(srv.URL + "/api/notifications/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("status = %d, Retry-After = %q", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
}

func TestStreamHandler_ReleasesTheClientWhenItLeaves(t *testing.T) {
	hub := newTestHub(HubConfig{})
	s := openStream(t, newStreamServer(t, hub, 0), "", nil)
	s.next(t)
	waitClients(t, hub, 1)

	s.resp.Body.Close()
	publishNotices(t, hub, 1) // la escritura falla o el contexto ya terminó
	waitClients(t, hub, 0)
}

// Los plazos del servidor (ReadTimeout de cmd/platform, o un WriteTimeout
// futuro) no cortan el stream.
func TestStreamHandler_SurvivesServerTimeouts(t *testing.T) {
	hub := newTestHub(HubConfig{})
	s := openStream(t, newStreamServer(t, hub, 50*time.Millisecond), "", nil)
	s.next(t)
	waitClients(t, hub, 1)

	time.Sleep(200 * time.Millisecond)
	if err := hub.DeliverChange(context.Background(), entities.AlertChange{AlertCode: "ALR-000001", Change: "OPENED"}); err != nil {
		t.Fatal(err)
	}
	if got := s.next(t); !strings.Contains(got, "event: alert.changed") {
		t.Fatalf("bloque = %q", got)
	}
}

func TestStreamHandler_ClosesWhenTheHubShutsDown(t *testing.T) {
	hub := newTestHub(HubConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { hub.Run(ctx); close(done) }()
	srv := newStreamServer(t, hub, 0)

	s := openStream(t, srv, "", nil)
	s.next(t)
	waitClients(t, hub, 1)
	cancel()
	<-done
	s.closed(t)

	resp, err := srv.Client().Get(srv.URL + "/api/notifications/stream")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d: el hub apagado no acepta conexiones", resp.StatusCode)
	}
}
