package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(
		postgres.Open(dsn),
		&gorm.Config{
			Logger: newGormLogger(),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// newGormLogger es logger.Default en nivel Error, pero registra el SQL sin los
// valores ($1, $2…): una consulta que falla no puede dejar en el log tokens
// FCM u otros datos sensibles. "record not found" no se registra: los
// repositorios lo traducen a nil (p. ej. la política por punto que cae a la
// global) y no es un error.
func newGormLogger() logger.Interface {
	return logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Error,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
		ParameterizedQueries:      true,
	})
}
