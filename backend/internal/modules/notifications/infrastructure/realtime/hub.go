package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// ErrTooManyClients: el hub llegó al tope de clientes simultáneos.
var ErrTooManyClients = errors.New("realtime: demasiados clientes conectados")

// Valores por defecto de HubConfig.
const (
	DefaultMaxClients        = 200
	DefaultReplaySize        = 256
	DefaultClientBuffer      = 32
	DefaultHeartbeatInterval = 25 * time.Second
)

// HubConfig son los límites del hub. Un valor en cero toma el de por defecto.
type HubConfig struct {
	// MaxClients es el tope de conexiones simultáneas (pestañas abiertas).
	MaxClients int
	// ReplaySize es cuántos eventos se guardan para reenviar al reconectar.
	ReplaySize int
	// ClientBuffer es cuántos frames puede tener pendientes un cliente antes
	// de que el hub lo desconecte por lento.
	ClientBuffer int
	// HeartbeatInterval es cada cuánto se manda ": ping".
	HeartbeatInterval time.Duration
}

func (c HubConfig) withDefaults() HubConfig {
	if c.MaxClients <= 0 {
		c.MaxClients = DefaultMaxClients
	}
	if c.ReplaySize <= 0 {
		c.ReplaySize = DefaultReplaySize
	}
	if c.ClientBuffer <= 0 {
		c.ClientBuffer = DefaultClientBuffer
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = DefaultHeartbeatInterval
	}
	return c
}

// Hub es el canal web: numera cada evento como <epoch>-<seq>, guarda los
// últimos en un buffer circular y los reparte a los clientes conectados sin
// bloquearse (un cliente lento se desconecta y se resincroniza al volver).
//
// El epoch cambia cada vez que arranca el proceso: un id de otro epoch no se
// puede reenviar y el cliente recibe resync.
type Hub struct {
	cfg   HubConfig
	epoch string
	log   *logger.Logger

	mu      sync.Mutex
	seq     uint64
	replay  []Frame // buffer circular; replay[(seq-1) % ReplaySize] es el último
	clients map[*Client]struct{}
}

var (
	_ ports.NoticeChannel = (*Hub)(nil)
	_ ports.ChangeChannel = (*Hub)(nil)
)

// NewHub crea el hub. Run arranca el heartbeat.
func NewHub(cfg HubConfig, log *logger.Logger) *Hub {
	cfg = cfg.withDefaults()
	return &Hub{
		cfg:     cfg,
		epoch:   strconv.FormatInt(time.Now().UnixMilli(), 10),
		log:     log,
		replay:  make([]Frame, cfg.ReplaySize),
		clients: make(map[*Client]struct{}),
	}
}

// Client es una conexión abierta. Frames se cierra cuando el hub la suelta
// (cliente lento o apagado): el handler debe terminar la respuesta.
type Client struct {
	frames chan Frame
}

// Frames son los eventos en vivo, en orden.
func (c *Client) Frames() <-chan Frame { return c.frames }

// Connect registra un cliente. lastEventID es el último id que vio (cabecera
// Last-Event-ID o ?last_event_id=); vacío si es la primera conexión.
// pending son los frames que debe escribir antes de leer Frames: lo que se
// perdió, o un resync si el id ya no está en el buffer o es de otro epoch.
func (h *Hub) Connect(lastEventID string) (c *Client, pending []Frame, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) >= h.cfg.MaxClients {
		return nil, nil, ErrTooManyClients
	}
	c = &Client{frames: make(chan Frame, h.cfg.ClientBuffer)}
	h.clients[c] = struct{}{}
	return c, h.missedSince(lastEventID), nil
}

// Disconnect suelta al cliente. Se puede llamar más de una vez.
func (h *Hub) Disconnect(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drop(c)
}

// Clients es la cantidad de clientes conectados.
func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Run manda el heartbeat hasta que ctx termina; al terminar suelta a todos los
// clientes para que sus respuestas se cierren.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(h.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				h.drop(c)
			}
			h.mu.Unlock()
			return
		case <-ticker.C:
			h.mu.Lock()
			h.broadcast(pingFrame)
			h.mu.Unlock()
		}
	}
}

// DeliverNotice publica alert.notification.
func (h *Hub) DeliverNotice(_ context.Context, n entities.AlertNotice) error {
	return h.publish(EventAlertNotification, toNoticePayload(n))
}

// DeliverChange publica alert.changed con el estado actual de la alerta.
func (h *Hub) DeliverChange(_ context.Context, c entities.AlertChange) error {
	return h.publish(EventAlertChanged, toChangePayload(c))
}

func (h *Hub) publish(event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("realtime: serializar %s: %w", event, err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	f := Frame{ID: formatEventID(h.epoch, h.seq), Event: event, Data: data}
	h.replay[(h.seq-1)%uint64(len(h.replay))] = f
	h.broadcast(f)
	return nil
}

// broadcast entrega sin bloquear. Requiere h.mu.
func (h *Hub) broadcast(f Frame) {
	for c := range h.clients {
		select {
		case c.frames <- f:
		default:
			h.drop(c)
			h.log.Warn("[Hub] Cliente lento desconectado; se resincronizará al reconectar",
				"buffer", h.cfg.ClientBuffer, "clients", len(h.clients))
		}
	}
}

// drop cierra y olvida al cliente. Requiere h.mu.
func (h *Hub) drop(c *Client) {
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	close(c.frames)
}

// missedSince devuelve lo publicado después de lastEventID. Requiere h.mu.
func (h *Hub) missedSince(lastEventID string) []Frame {
	if lastEventID == "" {
		return nil
	}
	epoch, last, ok := parseEventID(lastEventID)
	if !ok || epoch != h.epoch || last > h.seq {
		return []Frame{resyncFrame}
	}
	size := uint64(len(h.replay))
	if h.seq-last > size {
		return []Frame{resyncFrame}
	}
	missed := make([]Frame, 0, h.seq-last)
	for s := last + 1; s <= h.seq; s++ {
		missed = append(missed, h.replay[(s-1)%size])
	}
	return missed
}
