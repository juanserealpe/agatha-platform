package dtos

import "time"

type IngestRequest struct {
	DeviceID  string                 `json:"device_id" binding:"required"`
	Timestamp time.Time              `json:"timestamp" binding:"required"`
	Payload   map[string]interface{} `json:"payload" binding:"required"`
}
