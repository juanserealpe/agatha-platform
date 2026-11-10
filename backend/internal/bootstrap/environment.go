package bootstrap

import (
	"os"
	"strings"
)

// EnvironmentEnv es la variable con el entorno de despliegue. Solo importa si
// vale "production": ahí se bloquean el -rebuild de Rule Engine y de Alert
// Management y el endpoint de eventos de prueba de Notifications. Cualquier
// otro valor, o la variable sin definir, es "no producción".
const EnvironmentEnv = "AGATHA_ENV"

// productionEnvironment es el valor de AGATHA_ENV que activa los bloqueos.
const productionEnvironment = "production"

// isProductionEnvironment compara AGATHA_ENV sin distinguir mayúsculas ni
// espacios, con el mismo criterio que los servicios de backfill de Rule Engine
// y Alert Management.
func isProductionEnvironment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvironmentEnv)), productionEnvironment)
}
