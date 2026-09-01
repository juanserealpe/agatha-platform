package repositories

import (
	"context"

	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"gorm.io/gorm"
)

type UnitOfWork struct {
	db *gorm.DB
}

func NewUnitOfWork(
	db *gorm.DB,
) *UnitOfWork {
	return &UnitOfWork{
		db: db,
	}
}

func (u *UnitOfWork) Execute(
	ctx context.Context,
	fn func(
		ctx context.Context,
		measurementRepository domainRepositories.MeasurementRepository,
		ingestionStateRepository domainRepositories.IngestionStateRepository,
	) error,
) error {

	return u.db.WithContext(ctx).Transaction(
		func(tx *gorm.DB) error {

			measurementRepository :=
				NewMeasurementRepository(tx)

			ingestionStateRepository :=
				NewIngestionStateRepository(tx)

			return fn(
				ctx,
				measurementRepository,
				ingestionStateRepository,
			)
		},
	)
}
