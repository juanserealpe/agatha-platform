package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// ToBaselineSnapshot convierte una versión de línea base (con sus estadísticas
// precargadas) en la vista de Processing. FirstSeenAt no viene de esta tabla:
// lo completa el servicio.
func ToBaselineSnapshot(m models.BaselineVersionReadModel) entities.BaselineSnapshot {
	snap := entities.BaselineSnapshot{
		Available:        true,
		VersionID:        m.ID,
		Version:          m.Version,
		ValidFrom:        m.ValidFrom.UTC(),
		Confidence:       m.Confidence,
		Signal:           features.Signal{MedianB2: m.MedianB2, ThetaD: m.ThetaD, DeltaMin: m.DeltaMinB2},
		Stats:            map[int]features.ComponentStats{},
		Flags:            nonNil(m.Flags),
		AlgorithmVersion: m.AlgorithmVersion,
	}

	if m.ValidTo != nil {
		validTo := m.ValidTo.UTC()
		snap.ValidTo = &validTo
	}
	if m.P90Planitud != nil {
		snap.Signal.P90Planitud, snap.Signal.HasP90Planitud = *m.P90Planitud, true
	}
	if m.MinElevatedB2 != nil {
		snap.Signal.MinElevatedB2, snap.Signal.HasMinElevated = *m.MinElevatedB2, true
	}

	for _, c := range m.ComponentStats {
		hours := int(c.WindowHours)
		st := snap.Stats[hours]
		stat := features.Stat{Median: c.Median, MAD: c.MAD, N: c.NWindows}

		switch c.Component {
		case features.ComponentD:
			st.D = stat
		case features.ComponentP:
			st.P = stat
		case features.ComponentEx:
			st.Ex = stat
		case features.ComponentC:
			st.C, st.HasC = stat, true
		}

		snap.Stats[hours] = st
	}

	return snap
}
