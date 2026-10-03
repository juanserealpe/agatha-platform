package entities

import "github.com/juanserealpe/agatha-platform/backend/internal/shared/features"

// ComponentStat es la estadística histórica de un componente en una longitud
// de ventana. MAD es crudo (sin 1,4826 ni piso).
type ComponentStat struct {
	WindowHours int    // 6 | 24
	Component   string // features.ComponentD | P | EX | C
	Median      float64
	MAD         float64
	NWindows    int
}

// FeatureStats arma las estadísticas del kernel para una longitud de ventana.
// Un componente ausente queda con N = 0 (no calculable en features.Score).
func FeatureStats(stats []ComponentStat, windowHours int) features.ComponentStats {
	var out features.ComponentStats

	for _, st := range stats {
		if st.WindowHours != windowHours {
			continue
		}

		fs := features.Stat{Median: st.Median, MAD: st.MAD, N: st.NWindows}
		switch st.Component {
		case features.ComponentD:
			out.D = fs
		case features.ComponentP:
			out.P = fs
		case features.ComponentEx:
			out.Ex = fs
		case features.ComponentC:
			out.C = fs
			out.HasC = true
		}
	}

	return out
}
