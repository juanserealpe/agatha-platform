package bootstrap

import (
	"gorm.io/gorm"

	baselinePorts "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/knownevents"
	baselineRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// alertsTable es la tabla de Alert Management que Baseline lee con su propio
// modelo (acople de esquema, solo lectura; Alert Management no sabe de Baseline).
const alertsTable = "alerts"

// knownEventsProvider arma las fuentes de eventos conocidos de Baseline: su
// lista estática siempre y, si la tabla alerts existe, todas las alertas del
// punto (cualquier clasificación, también las activas). Se decide al arrancar: habilitar
// Alert Management después requiere reiniciar.
func knownEventsProvider(db *gorm.DB, log *logger.Logger) baselinePorts.KnownEventsProvider {
	static := knownevents.NewStaticProvider()
	if !db.Migrator().HasTable(alertsTable) {
		log.Info("eventos conocidos de la línea base: lista estática (no existe la tabla alerts)")
		return static
	}
	log.Info("eventos conocidos de la línea base: lista estática + todas las alertas")
	return knownevents.NewCompositeProvider(static, baselineRepositories.NewAlertEventsReader(db))
}
