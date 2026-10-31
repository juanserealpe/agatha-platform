package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/infrastructure/persistence/models"
)

// PushDeviceRepository implementa ports.PushTokenRepository sobre push_devices.
// Las consultas por token filtran por md5(token) para usar ux_push_devices_token.
type PushDeviceRepository struct {
	db *gorm.DB
}

func NewPushDeviceRepository(db *gorm.DB) *PushDeviceRepository {
	return &PushDeviceRepository{db: db}
}

// Upsert quita el token de otro celular que lo tuviera y guarda el de este, en
// una transacción. Si dos celulares registran el mismo token a la vez, uno
// falla por el índice único y la app reintenta.
func (r *PushDeviceRepository) Upsert(ctx context.Context, d entities.PushDevice) error {
	row := mappers.ToPushDeviceModel(d)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM push_devices WHERE md5(token) = md5(?) AND token = ? AND device_id <> ?`,
			row.Token, row.Token, row.DeviceID).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO push_devices (device_id, token, platform, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (device_id) DO UPDATE
			SET token = EXCLUDED.token, platform = EXCLUDED.platform, updated_at = EXCLUDED.updated_at`,
			row.DeviceID, row.Token, row.Platform, row.UpdatedAt).Error
	})
}

func (r *PushDeviceRepository) List(ctx context.Context) ([]entities.PushDevice, error) {
	var rows []models.PushDeviceModel
	if err := r.db.WithContext(ctx).Order("device_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]entities.PushDevice, len(rows))
	for i, row := range rows {
		out[i] = mappers.ToPushDevice(row)
	}
	return out, nil
}

func (r *PushDeviceRepository) DeleteToken(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Exec(`DELETE FROM push_devices WHERE md5(token) = md5(?) AND token = ?`, token, token).Error
}
