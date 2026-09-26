package dtos

import "time"

type MeasurementsResponse struct {
	DeviceID     int64                        `json:"deviceId"`
	StartDate    time.Time                    `json:"startDate"`
	EndDate      time.Time                    `json:"endDate"`
	Measurements []MetricMeasurementsResponse `json:"measurements"`
}
