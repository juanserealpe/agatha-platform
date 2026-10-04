package bootstrap

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
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

// NewPlatform carga el entorno, conecta la BD y crea el bus de eventos.
//
// El bus no levanta workers hasta que se registra una suscripción, por lo que
// crearlo acá no deja goroutines vivas si un paso posterior falla.
func NewPlatform() (*Platform, error) {
	return newPlatform(func(dsn string) string { return dsn })
}

// NewReadOnlyPlatform es NewPlatform con todas las transacciones de la sesión
// en solo lectura (default_transaction_read_only=on): PostgreSQL rechaza
// cualquier escritura. La usa cmd/replay.
func NewReadOnlyPlatform() (*Platform, error) {
	return newPlatform(readOnlyDSN)
}

func newPlatform(dsnFor func(string) string) (*Platform, error) {
	logDB := logger.New("DataBaseInit", "Info")

	if err := godotenv.Load(); err != nil {
		logDB.Warn("no se encontró archivo .env; se utilizarán variables de entorno")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logDB.Error("DATABASE_URL no está configurada")
		return nil, fmt.Errorf("DATABASE_URL no está configurada")
	}

	db, err := database.NewPostgres(dsnFor(dsn))
	if err != nil {
		logDB.Error("falló la conexión con la base de datos", "error", err)
		return nil, err
	}

	return &Platform{
		DB:       db,
		EventBus: eventbus.NewInMemory(logger.New("EventBus", "Info")),
	}, nil
}

// readOnlyDSN agrega el parámetro de sesión default_transaction_read_only=on
// (pgx lo envía como runtime param) a un DSN en formato URL o clave=valor.
func readOnlyDSN(dsn string) string {
	const param = "default_transaction_read_only"

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if u, err := url.Parse(dsn); err == nil {
			q := u.Query()
			q.Set(param, "on")
			u.RawQuery = q.Encode()
			return u.String()
		}
	}

	return strings.TrimSpace(dsn) + " " + param + "=on"
}

// Close drena el bus de eventos hasta el deadline de ctx.
func (p *Platform) Close(ctx context.Context) error {
	return p.EventBus.Close(ctx)
}
