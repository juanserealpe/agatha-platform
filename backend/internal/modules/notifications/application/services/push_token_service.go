package services

import (
	"context"
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// RegisterPushTokenCommand es el registro que manda la app al obtener o
// renovar su token FCM (README.md §5.2).
type RegisterPushTokenCommand struct {
	DeviceID string
	Token    string
	Platform string
}

// PushTokenService guarda el token vigente de cada celular.
type PushTokenService struct {
	repo  ports.PushTokenRepository
	clock ports.Clock
	log   *logger.Logger
}

func NewPushTokenService(repo ports.PushTokenRepository, clock ports.Clock, log *logger.Logger) *PushTokenService {
	return &PushTokenService{repo: repo, clock: clock, log: log}
}

// Register valida y guarda el token (upsert por deviceId, idempotente). Un
// registro inválido devuelve entities.ErrInvalidPushDevice. El token nunca va
// al log: solo su largo.
func (s *PushTokenService) Register(ctx context.Context, cmd RegisterPushTokenCommand) error {
	device, err := entities.NewPushDevice(cmd.DeviceID, cmd.Token, cmd.Platform, s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.repo.Upsert(ctx, device); err != nil {
		return fmt.Errorf("guardar el token de %s: %w", device.DeviceID, err)
	}
	s.log.Info("[PushTokenService] Token de push registrado",
		"device_id", device.DeviceID, "platform", device.Platform, "token_length", len(device.Token))
	return nil
}
