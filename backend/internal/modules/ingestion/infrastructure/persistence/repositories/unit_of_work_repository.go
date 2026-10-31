package repositories

import (
	"context"

	"gorm.io/gorm"

	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// UnitOfWork implementa ports.IngestionUnitOfWork sobre una transacción GORM.
type UnitOfWork struct {
	db     *gorm.DB
	logger *logger.Logger
}

func NewUnitOfWork(db *gorm.DB, logger *logger.Logger) *UnitOfWork {
	return &UnitOfWork{db: db, logger: logger}
}

func (u *UnitOfWork) Execute(
	ctx context.Context,
	fn func(
		ctx context.Context,
		measurementRepository domainRepositories.MeasurementRepository,
		ingestionStateRepository domainRepositories.IngestionStateRepository,
	) error,
) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, NewMeasurementRepository(tx, u.logger), NewIngestionStateRepository(tx))
	})
}
