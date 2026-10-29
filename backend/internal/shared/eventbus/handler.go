package eventbus

import "context"

// Handler procesa un evento entregado por el bus.
//
// El ctx recibido conserva los valores del publicador pero NO su cancelación,
// y expira según el timeout de la suscripción o al cerrar/cancelar la misma.
type Handler func(ctx context.Context, event any)
