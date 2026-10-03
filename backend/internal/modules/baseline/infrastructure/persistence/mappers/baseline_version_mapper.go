package mappers

import (
	"encoding/json"
	"sort"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

// ToBaselineVersionModel convierte la entidad al modelo (sin ID ni Version:
// los asigna el repositorio).
func ToBaselineVersionModel(v entities.BaselineVersion) (models.BaselineVersionModel, error) {
	periods := make([]models.ExcludedPeriodJSON, len(v.ExcludedPeriods))
	for i, p := range v.ExcludedPeriods {
		periods[i] = models.ExcludedPeriodJSON{From: p.From.UTC(), To: p.To.UTC(), PeakI: p.PeakI}
	}

	periodsJSON, err := json.Marshal(periods)
	if err != nil {
		return models.BaselineVersionModel{}, err
	}

	flags := make(models.TextArray, len(v.Flags))
	for i, f := range v.Flags {
		flags[i] = string(f)
	}

	stats := make([]models.BaselineComponentStatModel, len(v.Components))
	for i, st := range v.Components {
		stats[i] = models.BaselineComponentStatModel{
			WindowHours: int16(st.WindowHours),
			Component:   st.Component,
			Median:      st.Median,
			MAD:         st.MAD,
			NWindows:    st.NWindows,
		}
	}

	return models.BaselineVersionModel{
		MeasuringPointID:   v.MeasuringPointID,
		ValidFrom:          v.ValidFrom.UTC(),
		HistoryFrom:        v.HistoryFrom.UTC(),
		HistoryTo:          v.HistoryTo.UTC(),
		MedianB2:           v.Signal.MedianB2,
		ThetaD:             v.Signal.ThetaD,
		P90Planitud:        v.Signal.P90Planitud,
		MinElevatedB2:      v.Signal.MinElevatedB2,
		DeltaMinB2:         v.Signal.DeltaMin,
		NSignalRecords:     v.Signal.NRecords,
		NPlanitudRecords:   v.Signal.NPlanitudRecords,
		Confidence:         string(v.Confidence),
		ExclusionPasses:    int16(v.Estimation.Passes),
		ExclusionConverged: v.Estimation.Converged,
		ExcludedRatio:      v.Estimation.ExcludedRatio,
		ExcludedPeriods:    string(periodsJSON),
		Flags:              flags,
		Trigger:            string(v.Trigger),
		AlgorithmVersion:   v.AlgorithmVersion,
		CreatedAt:          v.CreatedAt.UTC(),
		ComponentStats:     stats,
	}, nil
}

// ToBaselineVersion convierte el modelo a la entidad.
func ToBaselineVersion(m models.BaselineVersionModel) (entities.BaselineVersion, error) {
	var periodsJSON []models.ExcludedPeriodJSON
	if m.ExcludedPeriods != "" {
		if err := json.Unmarshal([]byte(m.ExcludedPeriods), &periodsJSON); err != nil {
			return entities.BaselineVersion{}, err
		}
	}

	periods := make([]entities.ExcludedPeriod, len(periodsJSON))
	for i, p := range periodsJSON {
		periods[i] = entities.ExcludedPeriod{From: p.From.UTC(), To: p.To.UTC(), PeakI: p.PeakI}
	}

	flags := make([]entities.Flag, len(m.Flags))
	for i, f := range m.Flags {
		flags[i] = entities.Flag(f)
	}

	// Orden estable: 6 h antes que 24 h y D, P, EX, C.
	order := map[string]int{"D": 0, "P": 1, "EX": 2, "C": 3}
	stats := make([]entities.ComponentStat, len(m.ComponentStats))
	for i, st := range m.ComponentStats {
		stats[i] = entities.ComponentStat{
			WindowHours: int(st.WindowHours),
			Component:   st.Component,
			Median:      st.Median,
			MAD:         st.MAD,
			NWindows:    st.NWindows,
		}
	}
	sort.SliceStable(stats, func(a, b int) bool {
		if stats[a].WindowHours != stats[b].WindowHours {
			return stats[a].WindowHours < stats[b].WindowHours
		}
		return order[stats[a].Component] < order[stats[b].Component]
	})

	var validTo = m.ValidTo
	if validTo != nil {
		utc := validTo.UTC()
		validTo = &utc
	}

	return entities.BaselineVersion{
		ID:               m.ID,
		MeasuringPointID: m.MeasuringPointID,
		Version:          m.Version,
		ValidFrom:        m.ValidFrom.UTC(),
		ValidTo:          validTo,
		HistoryFrom:      m.HistoryFrom.UTC(),
		HistoryTo:        m.HistoryTo.UTC(),
		Signal: entities.SignalBaseline{
			MedianB2:         m.MedianB2,
			ThetaD:           m.ThetaD,
			P90Planitud:      m.P90Planitud,
			MinElevatedB2:    m.MinElevatedB2,
			DeltaMin:         m.DeltaMinB2,
			NRecords:         m.NSignalRecords,
			NPlanitudRecords: m.NPlanitudRecords,
		},
		Components:      stats,
		Confidence:      entities.Confidence(m.Confidence),
		ExcludedPeriods: periods,
		Estimation: entities.EstimationSummary{
			Passes:        int(m.ExclusionPasses),
			Converged:     m.ExclusionConverged,
			ExcludedRatio: m.ExcludedRatio,
			CapReached:    entities.HasFlag(flags, entities.FlagExclusionCapReached),
		},
		Flags:            flags,
		Trigger:          entities.Trigger(m.Trigger),
		AlgorithmVersion: m.AlgorithmVersion,
		CreatedAt:        m.CreatedAt.UTC(),
	}, nil
}
