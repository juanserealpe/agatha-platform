package mapping

import (
	"strings"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
)

type MetricDefinition struct {
	Key      string
	Name     string
	Category enums.MetricCategory
}

type MetricCatalog struct {
	metrics map[string]MetricDefinition
}

func NewMetricCatalog() *MetricCatalog {
	return &MetricCatalog{
		metrics: map[string]MetricDefinition{

			// Sensors
			"Pitch": {
				Key:      "pitch",
				Name:     "Pitch",
				Category: enums.CategorySensor,
			},
			"Roll": {
				Key:      "roll",
				Name:     "Roll",
				Category: enums.CategorySensor,
			},
			"Voltaje (mV)": {
				Key:      "voltaje",
				Name:     "Voltaje",
				Category: enums.CategorySensor,
			},
			"Corriente (mA)": {
				Key:      "corriente",
				Name:     "Corriente",
				Category: enums.CategorySensor,
			},
			"Batería (%)": {
				Key:      "bateria",
				Name:     "Batería",
				Category: enums.CategorySensor,
			},

			// Features
			"RMS (ADC)": {
				Key:      "rms",
				Name:     "RMS",
				Category: enums.CategoryFeature,
			},
			"Centroide espectral (Hz)": {
				Key:      "centroide_espectral",
				Name:     "Centroide espectral",
				Category: enums.CategoryFeature,
			},
			"Planitud espectral": {
				Key:      "planitud_espectral",
				Name:     "Planitud espectral",
				Category: enums.CategoryFeature,
			},
			"Deriva (grados)": {
				Key:      "deriva",
				Name:     "Deriva",
				Category: enums.CategoryFeature,
			},
			"Actividad": {
				Key:      "actividad",
				Name:     "Actividad",
				Category: enums.CategoryFeature,
			},
			"Mag promedio (g)": {
				Key:      "mag_promedio",
				Name:     "Indicadores de fuga acustica",
				Category: enums.CategoryFeature,
			},
			"Act. pico (g/s)": {
				Key:      "act_pico",
				Name:     "Act. pico",
				Category: enums.CategoryFeature,
			},
			"Razon R (fuga/viento)": {
				Key:      "razib_r",
				Name:     "Razib R",
				Category: enums.CategoryFeature,
			},
			"B0 viento 30-300Hz": {
				Key:      "b0_viento_30_300hz",
				Name:     "B0 viento 30-300Hz",
				Category: enums.CategoryFeature,
			},
			"B1 fuga 500-1200Hz": {
				Key:      "b1_fuga_500_1200hz",
				Name:     "B1 fuga 500-1200Hz",
				Category: enums.CategoryFeature,
			},
			"B2 fuga 1200-2000Hz": {
				Key:      "b2_fuga_1200_2000hz",
				Name:     "B2 fuga 1200-2000Hz",
				Category: enums.CategoryFeature,
			},
			"B3 guarda 2-2.4kHz": {
				Key:      "b3_guarda_2_2_4khz",
				Name:     "B3 guarda 2-2.4kHz",
				Category: enums.CategoryFeature,
			},
			"Pitch base": {
				Key:      "pitch_base",
				Name:     "Pitch base",
				Category: enums.CategoryFeature,
			},
			"Roll base": {
				Key:      "roll_base",
				Name:     "Roll base",
				Category: enums.CategoryFeature,
			},
			"RMS acustico": {
				Key:      "rms_acustico",
				Name:     "RMS acustico",
				Category: enums.CategoryFeature,
			},
			// Status
			"Fuga sospechada": {
				Key:      "fuga_sospechada",
				Name:     "Fuga sospechada",
				Category: enums.CategoryStatus,
			},
		},
	}
}

func (c *MetricCatalog) Find(
	externalName string,
) (MetricDefinition, bool) {

	metric, ok := c.metrics[externalName]

	return metric, ok
}

func (c *MetricCatalog) FindNormalized(
	externalName string,
) (MetricDefinition, bool) {

	metric, ok := c.Find(externalName)
	if ok {
		return metric, true
	}

	normalized := strings.TrimSpace(externalName)

	for key, definition := range c.metrics {
		if strings.EqualFold(key, normalized) {
			return definition, true
		}
	}

	return MetricDefinition{}, false
}
