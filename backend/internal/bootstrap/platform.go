package bootstrap

import (
	"context"
	"fmt"
	"os"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/eventbus"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Platform agrupa la infraestructura compartida por todos los módulos del
// monolito: una sola BD y un solo bus de eventos en proceso.
//
// El bus es en memoria: publicador y consumidores DEBEN vivir en el mismo
// proceso que comparte esta Platform.
type Platform struct {
	DB       *gorm.DB
	EventBus *eventbus.InMemoryBus
}

// NewPlatform carga el entorno (LoadEnvironment: AGATHA_DB elige la BD),
// conecta la BD y crea el bus de eventos.
//
// El bus no levanta workers hasta que se registra una suscripción, por lo que
// crearlo acá no deja goroutines vivas si un paso posterior falla.
func NewPlatform() (*Platform, error) {
	logDB := logger.New("DataBaseInit", "Info")

	target, err := LoadEnvironment(".")
	if err != nil {
		logDB.Error("no se pudo cargar el entorno", "error", err)
		return nil, err
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logDB.Error("DATABASE_URL no está configurada", "ayuda", "definir AGATHA_DB=local|production o DATABASE_URL")
		return nil, fmt.Errorf("DATABASE_URL no está configurada")
	}

	// Siempre queda en el log contra qué BD corre el proceso.
	selected := string(target)
	if target == DatabaseTargetUnset {
		selected = "sin AGATHA_DB (.env / entorno)"
	}
	logDB.Info("base de datos seleccionada", "destino", selected, "bd", DescribeDatabaseURL(dsn))

	db, err := database.NewPostgres(dsn)
	if err != nil {
		logDB.Error("falló la conexión con la base de datos", "error", err)
		return nil, err
	}

	return &Platform{
		DB:       db,
		EventBus: eventbus.NewInMemory(logger.New("EventBus", "Info")),
	}, nil
}

// Close drena el bus de eventos hasta el deadline de ctx.
func (p *Platform) Close(ctx context.Context) error {
	return p.EventBus.Close(ctx)
}
