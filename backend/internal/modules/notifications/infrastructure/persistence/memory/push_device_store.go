// Package memory es el almacenamiento en memoria de Notifications, con las
// mismas reglas que el de PostgreSQL. Lo usan las pruebas (sin base de datos).
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// PushDeviceStore implementa ports.PushTokenRepository en memoria.
type PushDeviceStore struct {
	mu      sync.Mutex
	devices map[string]entities.PushDevice
}

func NewPushDeviceStore(devices ...entities.PushDevice) *PushDeviceStore {
	s := &PushDeviceStore{devices: map[string]entities.PushDevice{}}
	for _, d := range devices {
		s.devices[d.DeviceID] = d
	}
	return s
}

func (s *PushDeviceStore) Upsert(_ context.Context, d entities.PushDevice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, other := range s.devices {
		if other.Token == d.Token && id != d.DeviceID {
			delete(s.devices, id)
		}
	}
	s.devices[d.DeviceID] = d
	return nil
}

func (s *PushDeviceStore) List(context.Context) ([]entities.PushDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]entities.PushDevice, 0, len(s.devices))
	for _, d := range s.devices {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out, nil
}

func (s *PushDeviceStore) DeleteToken(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, d := range s.devices {
		if d.Token == token {
			delete(s.devices, id)
		}
	}
	return nil
}
