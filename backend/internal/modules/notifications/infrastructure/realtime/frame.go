// Package realtime es el canal web de Notifications: un hub de Server-Sent
// Events que reparte los avisos y el estado actual de las alertas a las
// pestañas abiertas.
package realtime

import (
	"bytes"
	"strconv"
	"strings"
)

// Nombres de los eventos SSE que recibe el frontend.
const (
	EventAlertNotification = "alert.notification"
	EventAlertChanged      = "alert.changed"
	// EventResync le dice al cliente que perdió eventos y debe volver a pedir
	// /active y /pending por REST.
	EventResync = "resync"
)

// Frame es un mensaje SSE. Sin Event ni ID es el heartbeat (un comentario).
type Frame struct {
	// ID es <epoch>-<seq>; vacío en el heartbeat y en resync (no se reenvían).
	ID    string
	Event string
	Data  []byte
}

var (
	pingFrame   = Frame{}
	resyncFrame = Frame{Event: EventResync, Data: []byte("{}")}
)

// Bytes serializa el frame en el formato de text/event-stream.
func (f Frame) Bytes() []byte {
	if f.Event == "" && f.ID == "" {
		return []byte(": ping\n\n")
	}
	var b bytes.Buffer
	if f.ID != "" {
		b.WriteString("id: " + f.ID + "\n")
	}
	b.WriteString("event: " + f.Event + "\n")
	b.WriteString("data: ")
	b.Write(f.Data)
	b.WriteString("\n\n")
	return b.Bytes()
}

func formatEventID(epoch string, seq uint64) string {
	return epoch + "-" + strconv.FormatUint(seq, 10)
}

// parseEventID separa <epoch>-<seq>. false = el id no tiene ese formato.
func parseEventID(id string) (epoch string, seq uint64, ok bool) {
	i := strings.LastIndexByte(id, '-')
	if i <= 0 {
		return "", 0, false
	}
	seq, err := strconv.ParseUint(id[i+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return id[:i], seq, true
}
