// Package fcmtest tiene un ports.PushSender falso para las pruebas: graba lo
// enviado y devuelve los errores que se le programen por token.
package fcmtest

import (
	"context"
	"sync"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// Sent es un envío grabado.
type Sent struct {
	Token   string
	Message entities.PushMessage
}

// FakeSender graba cada intento de envío. Es seguro para uso concurrente.
type FakeSender struct {
	mu     sync.Mutex
	sent   []Sent
	errors map[string][]error
}

func NewFakeSender() *FakeSender {
	return &FakeSender{errors: map[string][]error{}}
}

// FailNext programa errores para los próximos envíos a token, en orden. Cuando
// se acaban, los envíos a ese token funcionan.
func (f *FakeSender) FailNext(token string, errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors[token] = append(f.errors[token], errs...)
}

// Send graba el intento (también los fallidos) y devuelve el siguiente error programado.
func (f *FakeSender) Send(_ context.Context, token string, m entities.PushMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, Sent{Token: token, Message: m})
	if queue := f.errors[token]; len(queue) > 0 {
		f.errors[token] = queue[1:]
		return queue[0]
	}
	return nil
}

// Sent devuelve una copia de los intentos grabados.
func (f *FakeSender) Sent() []Sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Sent(nil), f.sent...)
}

// Attempts cuenta los intentos a un token.
func (f *FakeSender) Attempts(token string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, s := range f.sent {
		if s.Token == token {
			n++
		}
	}
	return n
}
