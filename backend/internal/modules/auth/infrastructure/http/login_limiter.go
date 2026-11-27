package http

import (
	"sync"
	"time"
)

// LoginLimiter bloquea un cliente (por IP) tras maxFailures logins fallidos
// dentro de window. En memoria: se reinicia con el proceso, suficiente para
// frenar la fuerza bruta contra el único usuario.
type LoginLimiter struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	now         func() time.Time
	entries     map[string]*limiterEntry
}

type limiterEntry struct {
	failures int
	resetAt  time.Time
}

func NewLoginLimiter(maxFailures int, window time.Duration, now func() time.Time) *LoginLimiter {
	return &LoginLimiter{maxFailures: maxFailures, window: window, now: now, entries: map[string]*limiterEntry{}}
}

// Blocked devuelve si el cliente está bloqueado y cuánto falta.
func (l *LoginLimiter) Blocked(client string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.current(client)
	if entry == nil || entry.failures < l.maxFailures {
		return false, 0
	}
	return true, entry.resetAt.Sub(l.now())
}

// Fail registra un login fallido. La ventana empieza en el primer fallo.
func (l *LoginLimiter) Fail(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.current(client)
	if entry == nil {
		entry = &limiterEntry{resetAt: l.now().Add(l.window)}
		l.entries[client] = entry
	}
	entry.failures++
}

// Succeed limpia los fallos del cliente.
func (l *LoginLimiter) Succeed(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, client)
}

// current devuelve la entrada vigente y, de paso, purga las vencidas para que
// el mapa no crezca sin límite.
func (l *LoginLimiter) current(client string) *limiterEntry {
	now := l.now()
	for key, entry := range l.entries {
		if !now.Before(entry.resetAt) {
			delete(l.entries, key)
		}
	}
	return l.entries[client]
}
