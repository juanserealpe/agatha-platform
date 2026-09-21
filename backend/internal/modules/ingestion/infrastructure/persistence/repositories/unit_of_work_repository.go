package repositories

import (
	"context"

	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
	"gorm.io/gorm"
)

type UnitOfWork struct {
	db     *gorm.DB
	logger *logger.Logger
}

func NewUnitOfWork(
	db *gorm.DB,
	logger *logger.Logger,
) *UnitOfWork {
	return &UnitOfWork{
		db:     db,
		logger: logger,
	}
}

func (u *UnitOfWork) Execute(
	ctx context.Context,
	fn func(ctx context.Context, measurementRepository domainRepositories.MeasurementRepository,
		ingestionStateRepository domainRepositories.IngestionStateRepository, ingestionGapRepository domainRepositories.IngestionGapRepository,
	) error,
) error {

	return u.db.WithContext(ctx).Transaction(
		func(tx *gorm.DB) error {

			measurementRepository := NewMeasurementRepository(tx, u.logger)
			ingestionStateRepository := NewIngestionStateRepository(tx)
			ingestionGapRepository := NewGormIngestionGapRepository(tx)

			return fn(ctx, measurementRepository, ingestionStateRepository, ingestionGapRepository)
		},
	)
}
