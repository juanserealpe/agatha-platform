package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

// MetricDefinitionRepository define las operaciones de persistencia
// disponibles para las definiciones de métricas.
//
// Las implementaciones de esta interfaz son responsables de consultar
// y almacenar las definiciones de métricas en la capa de persistencia.
type MetricDefinitionRepository interface {

	// Save guarda una definición de métrica en el sistema.
	//
	// Recibe:
	//   - ctx: contexto de ejecución de la operación.
	//   - metric: definición de métrica que se desea almacenar.
	//
	// Devuelve:
	//   - error: nil si la operación fue exitosa; en caso contrario,
	//     devuelve el error producido durante la persistencia.
	Save(
		ctx context.Context,
		metric entities.MetricDefinition,
	) error

	// FindByID busca una definición de métrica utilizando su identificador.
	//
	// Recibe:
	//   - ctx: contexto de ejecución de la operación.
	//   - id: identificador único de la definición de métrica.
	//
	// Devuelve:
	//   - *entities.MetricDefinition: definición encontrada.
	//   - error: nil si la consulta fue exitosa; en caso contrario,
	//     devuelve el error producido durante la consulta.
	//
	// Si no existe una métrica con el ID proporcionado, la implementación
	// puede devolver nil como definición y nil como error.
	FindByID(
		ctx context.Context,
		id string,
	) (*entities.MetricDefinition, error)

	// FindByKey busca una definición de métrica utilizando su clave interna.
	//
	// Recibe:
	//   - ctx: contexto de ejecución de la operación.
	//   - key: clave única que identifica la métrica.
	//
	// Devuelve:
	//   - *entities.MetricDefinition: definición encontrada.
	//   - error: nil si la consulta fue exitosa; en caso contrario,
	//     devuelve el error producido durante la consulta.
	//
	// Si no existe una métrica con la clave proporcionada, la implementación
	// puede devolver nil como definición y nil como error.
	FindByKey(
		ctx context.Context,
		key string,
	) (*entities.MetricDefinition, error)

	// ListByDevice obtiene todas las definiciones de métricas asociadas
	// a un dispositivo.
	//
	// Recibe:
	//   - ctx: contexto de ejecución de la operación.
	//   - deviceID: identificador del dispositivo cuyas métricas se desean consultar.
	//
	// Devuelve:
	//   - []entities.MetricDefinition: lista de definiciones de métricas
	//     asociadas al dispositivo.
	//   - error: nil si la consulta fue exitosa; en caso contrario,
	//     devuelve el error producido durante la consulta.
	ListByDevice(
		ctx context.Context,
		deviceID string,
	) ([]entities.MetricDefinition, error)
}
