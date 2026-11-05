package eventbus

import "context"

// envelope es un evento encolado junto al contexto del publicador.
type envelope struct {
	ctx   context.Context
	event any
}
