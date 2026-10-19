package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
)

// LevelEventPublisher publica los eventos de nivel del Rule Engine. El
// adaptador hacia el bus vive en internal/bootstrap (el módulo no conoce el bus).
//
// Se invoca SIEMPRE después de persistir; un error nunca hace fallar la evaluación.
type LevelEventPublisher interface {
	PublishLeakThresholdExceeded(ctx context.Context, event contract.LeakThresholdExceeded) error
	PublishLeakThresholdCleared(ctx context.Context, event contract.LeakThresholdCleared) error
}
