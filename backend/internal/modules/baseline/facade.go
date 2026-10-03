// Package baseline expone la línea base por dispositivo (Device Context &
// Baseline). Su contrato público es Facade + types.go: solo tipos stdlib.
package baseline

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
)

// Facade es la consulta síncrona que consumirá Processing (vía su propio port
// y un adapter en internal/bootstrap).
type Facade interface {
	// GetEffectiveBaseline devuelve la versión con valid_from <= at < valid_to.
	// Ausencia ≠ error: Status = "NONE".
	GetEffectiveBaseline(ctx context.Context, measuringPointID int64, at time.Time) (EffectiveBaseline, error)
}

type facade struct {
	query *services.QueryService
}

// NewFacade crea la fachada sobre el QueryService.
func NewFacade(query *services.QueryService) Facade {
	return &facade{query: query}
}

func (f *facade) GetEffectiveBaseline(ctx context.Context, measuringPointID int64, at time.Time,
) (EffectiveBaseline, error) {
	result, err := f.query.GetEffective(ctx, measuringPointID, at)
	if err != nil {
		return EffectiveBaseline{}, err
	}

	return toEffectiveBaseline(measuringPointID, result), nil
}

func toEffectiveBaseline(pointID int64, result services.EffectiveResult) EffectiveBaseline {
	out := EffectiveBaseline{
		MeasuringPointID:  pointID,
		Status:            StatusNone,
		DeviceFirstSeenAt: result.FirstSeenAt,
		Components:        []ComponentStats{},
		Flags:             []string{},
	}

	v := result.Version
	if v == nil {
		return out
	}

	out.Status = StatusAvailable
	out.VersionID = v.ID
	out.Version = v.Version
	out.Confidence = string(v.Confidence)
	out.ValidFrom = v.ValidFrom
	out.AlgorithmVersion = v.AlgorithmVersion
	out.Signal = &SignalStats{
		MedianB2:      v.Signal.MedianB2,
		ThetaD:        v.Signal.ThetaD,
		P90Planitud:   copyFloat(v.Signal.P90Planitud),
		MinElevatedB2: copyFloat(v.Signal.MinElevatedB2),
		DeltaMin:      v.Signal.DeltaMin,
	}

	for _, st := range v.Components {
		out.Components = append(out.Components, ComponentStats{
			WindowHours: st.WindowHours,
			Component:   st.Component,
			Median:      st.Median,
			MAD:         st.MAD,
			NWindows:    st.NWindows,
		})
	}

	for _, f := range entities.NormalizeFlags(v.Flags) {
		out.Flags = append(out.Flags, string(f))
	}

	return out
}

func copyFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
