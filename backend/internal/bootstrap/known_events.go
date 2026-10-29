package bootstrap

import (
	"context"
	"time"

	"gorm.io/gorm"

	baselineEntities "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	baselinePorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	baselineRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// alertsTable es la tabla de Alert Management que Baseline lee con su propio
// modelo (acople de esquema, solo lectura; Alert Management no sabe de Baseline).
const alertsTable = "alerts"

// knownEventsProvider entrega a Baseline todas las alertas del punto
// (cualquier clasificación, también las activas). Si la tabla alerts no existe
// todavía no hay alertas que excluir. Se decide al arrancar: habilitar Alert
// Management después requiere reiniciar.
func knownEventsProvider(db *gorm.DB, log *logger.Logger) baselinePorts.KnownEventsProvider {
	if !db.Migrator().HasTable(alertsTable) {
		log.Warn("no existe la tabla alerts: la línea base se calcula sin excluir alertas")
		return noKnownEvents{}
	}
	log.Info("eventos conocidos de la línea base: todas las alertas del punto")
	return baselineRepositories.NewAlertEventsReader(db)
}

// noKnownEvents es el provider sin alertas (Alert Management nunca migró).
type noKnownEvents struct{}

func (noKnownEvents) GetKnownEvents(context.Context, int64, time.Time, time.Time) ([]baselineEntities.KnownEvent, error) {
	return []baselineEntities.KnownEvent{}, nil
}
