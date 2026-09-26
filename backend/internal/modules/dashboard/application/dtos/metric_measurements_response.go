package dtos

type MetricMeasurementsResponse struct {
	MetricID   uint64                     `json:"metricId"`
	MetricName string                     `json:"metricName"`
	Unit       string                     `json:"unit"`
	Data       []MeasurementPointResponse `json:"data"`
}
