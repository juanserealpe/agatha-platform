package mappers

import (
	"math"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// MetricIDs son los IDs del catálogo que usa Baseline.
type MetricIDs struct {
	B1, B2, Planitud uint64
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// ToSamples pivota filas (timestamp, metric_id, value) ordenadas por timestamp
// a un Sample por timestamp. Valores NaN/±Inf ⇒ Has* = false.
func ToSamples(rows []models.RawSampleRowModel, ids MetricIDs) []features.Sample {
	out := make([]features.Sample, 0, len(rows)/3+1)

	for _, row := range rows {
		last := len(out) - 1
		if last < 0 || !out[last].T.Equal(row.Timestamp) {
			out = append(out, features.Sample{T: row.Timestamp.UTC()})
			last++
		}

		s := &out[last]
		ok := finite(row.Value)
		switch row.MetricID {
		case ids.B1:
			s.B1, s.HasB1 = row.Value, ok
		case ids.B2:
			s.B2, s.HasB2 = row.Value, ok
		case ids.Planitud:
			s.Pl, s.HasPl = row.Value, ok
		}
	}

	return out
}
