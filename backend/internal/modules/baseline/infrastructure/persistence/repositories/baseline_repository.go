package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

// uniqueViolation es el SQLSTATE de PostgreSQL para violación de índice único.
const uniqueViolation = "23505"

// errNonMonotonicValidFrom: la nueva versión no puede empezar antes (o en el
// mismo instante) que la vigente; el versionado no es retroactivo.
var errNonMonotonicValidFrom = errors.New("valid_from de la nueva versión no es posterior al de la vigente")

// errForeignVersion: SaveHistory recibió una versión de otro punto.
var errForeignVersion = errors.New("la versión no pertenece al punto del histórico")

type BaselineRepository struct {
	db *gorm.DB
}

func NewBaselineRepository(db *gorm.DB) *BaselineRepository {
	return &BaselineRepository{db: db}
}

// GetEffective devuelve la versión con valid_from <= at < valid_to (nil si no hay).
func (r *BaselineRepository) GetEffective(ctx context.Context, pointID int64, at time.Time,
) (*entities.BaselineVersion, error) {
	at = at.UTC()

	return r.findOne(ctx, func(q *gorm.DB) *gorm.DB {
		return q.
			Where("measuring_point_id = ? AND valid_from <= ?", pointID, at).
			Where("(valid_to IS NULL OR valid_to > ?)", at).
			Order("valid_from DESC")
	})
}

// GetLatest devuelve la versión más reciente del punto (nil si no hay).
func (r *BaselineRepository) GetLatest(ctx context.Context, pointID int64) (*entities.BaselineVersion, error) {
	return r.findOne(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("measuring_point_id = ?", pointID).Order("version DESC")
	})
}

func (r *BaselineRepository) findOne(ctx context.Context, scope func(*gorm.DB) *gorm.DB,
) (*entities.BaselineVersion, error) {
	var rows []models.BaselineVersionModel

	err := scope(r.db.WithContext(ctx).Preload("ComponentStats")).
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, nil
	}

	version, err := mappers.ToBaselineVersion(rows[0])
	if err != nil {
		return nil, err
	}

	return &version, nil
}

// SaveNewVersion, en una TX: SELECT vigente FOR UPDATE → valid_to = v.ValidFrom
// → INSERT versión (anterior + 1) → INSERT component stats. Si otro proceso
// creó la vigente (índice parcial o UNIQUE (punto, versión)) devuelve
// domain.ErrBaselineConcurrentUpdate.
func (r *BaselineRepository) SaveNewVersion(ctx context.Context, v entities.BaselineVersion,
) (entities.BaselineVersion, error) {
	model, err := mappers.ToBaselineVersionModel(v)
	if err != nil {
		return entities.BaselineVersion{}, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current []models.BaselineVersionModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("measuring_point_id = ? AND valid_to IS NULL", v.MeasuringPointID).
			Limit(1).
			Find(&current).Error; err != nil {
			return err
		}

		if len(current) == 1 {
			if !model.ValidFrom.After(current[0].ValidFrom) {
				return errNonMonotonicValidFrom
			}
			if err := tx.Model(&models.BaselineVersionModel{}).
				Where("id = ?", current[0].ID).
				Update("valid_to", model.ValidFrom).Error; err != nil {
				return err
			}
		}

		// Versión = máxima del punto + 1 (no depende de que exista una vigente).
		var maxVersion int
		if err := tx.Model(&models.BaselineVersionModel{}).
			Where("measuring_point_id = ?", v.MeasuringPointID).
			Select("COALESCE(MAX(version), 0)").
			Scan(&maxVersion).Error; err != nil {
			return err
		}
		model.Version = maxVersion + 1

		// Inserta la versión y, por la asociación, sus component stats.
		return tx.Create(&model).Error
	})

	if err != nil {
		if isUniqueViolation(err) {
			return entities.BaselineVersion{}, domain.ErrBaselineConcurrentUpdate
		}
		return entities.BaselineVersion{}, err
	}

	return mappers.ToBaselineVersion(model)
}

// SaveHistory, en una TX: verifica que el punto no tenga versiones → INSERT de
// cada versión (número = posición, valid_to = valid_from de la siguiente; la
// última queda vigente) con sus component stats. Todo o nada.
func (r *BaselineRepository) SaveHistory(ctx context.Context, pointID int64, versions []entities.BaselineVersion,
) ([]entities.BaselineVersion, error) {
	saved := make([]models.BaselineVersionModel, len(versions))

	for i, v := range versions {
		if v.MeasuringPointID != pointID {
			return nil, errForeignVersion
		}
		if i > 0 && !v.ValidFrom.After(versions[i-1].ValidFrom) {
			return nil, errNonMonotonicValidFrom
		}

		model, err := mappers.ToBaselineVersionModel(v)
		if err != nil {
			return nil, err
		}
		model.Version = i + 1
		if i+1 < len(versions) {
			validTo := versions[i+1].ValidFrom.UTC()
			model.ValidTo = &validTo
		}
		saved[i] = model
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&models.BaselineVersionModel{}).
			Where("measuring_point_id = ?", pointID).
			Limit(1).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return domain.ErrBaselineHistoryExists
		}

		// Una por una y en orden: el índice parcial "una vigente por punto"
		// solo admite valid_to NULL en la última.
		for i := range saved {
			if err := tx.Create(&saved[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		// Dos cargas concurrentes: la segunda choca con UNIQUE (punto, versión).
		if isUniqueViolation(err) {
			return nil, domain.ErrBaselineHistoryExists
		}
		return nil, err
	}

	out := make([]entities.BaselineVersion, len(saved))
	for i, model := range saved {
		version, err := mappers.ToBaselineVersion(model)
		if err != nil {
			return nil, err
		}
		out[i] = version
	}

	return out, nil
}

// isUniqueViolation detecta el SQLSTATE 23505 sin acoplarse al driver.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == uniqueViolation
}
