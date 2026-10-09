package services

import (
	"strconv"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/dtos"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

const (
	callbackEvent       = "data_record"
	maxCallbackRecordID = 128

	// naiveTimestampLayout es el formato de Dataflow Labs sin zona horaria
	// (fracción de segundo opcional).
	naiveTimestampLayout = "2006-01-02T15:04:05.999999999"
)

// CallbackValidator valida el payload del callback y lo convierte en un
// CallbackRecord con los timestamps ya parseados.
type CallbackValidator struct {
	// location interpreta los timestamps sin zona horaria (America/Bogota,
	// la misma que se usa al consultar DataOS).
	location *time.Location
}

func NewCallbackValidator(location *time.Location) *CallbackValidator {
	return &CallbackValidator{location: location}
}

func (v *CallbackValidator) Validate(request dtos.DataRecordCallback) (*entities.CallbackRecord, error) {
	if request.Event != callbackEvent {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("event debe ser " + callbackEvent)
	}

	if strings.TrimSpace(request.RecordID) == "" {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("record_id es obligatorio")
	}
	if len(request.RecordID) > maxCallbackRecordID {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("record_id supera " + strconv.Itoa(maxCallbackRecordID) + " caracteres")
	}

	if request.MpID <= 0 {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("mp_id debe ser un entero positivo")
	}

	if strings.TrimSpace(request.SerialID) == "" {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("serial_id es obligatorio")
	}

	timestamp, ok := v.parseTimestamp(request.Timestamp)
	if !ok {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("timestamp inválido")
	}

	if len(request.Records) == 0 {
		return nil, ingestionErrors.NewInvalidCallbackPayloadError("records debe contener al menos una variable")
	}

	readings := make([]entities.CallbackReading, 0, len(request.Records))
	for i, item := range request.Records {
		field := "records[" + strconv.Itoa(i) + "]"

		if item.DeviceID != request.MpID {
			return nil, ingestionErrors.NewInvalidCallbackPayloadError(field + ".device_id debe coincidir con mp_id")
		}
		if strings.TrimSpace(item.Name) == "" {
			return nil, ingestionErrors.NewInvalidCallbackPayloadError(field + ".name es obligatorio")
		}
		if item.Readings == "" {
			return nil, ingestionErrors.NewInvalidCallbackPayloadError(field + ".readings es obligatorio")
		}

		readingTimestamp, ok := v.parseTimestamp(item.Timestamp)
		if !ok {
			return nil, ingestionErrors.NewInvalidCallbackPayloadError(field + ".timestamp inválido")
		}

		readings = append(readings, entities.CallbackReading{
			DeviceID:     item.DeviceID,
			Name:         item.Name,
			Timestamp:    readingTimestamp,
			RawTimestamp: item.Timestamp,
			Value:        string(item.Readings),
		})
	}

	return &entities.CallbackRecord{
		RecordID:         request.RecordID,
		MeasuringPointID: request.MpID,
		SerialID:         request.SerialID,
		Timestamp:        timestamp,
		Readings:         readings,
	}, nil
}

// parseTimestamp acepta RFC 3339 con offset explícito o el formato sin zona de
// Dataflow Labs, que se interpreta en v.location. El sufijo Z también se trata
// como hora local: Dataflow Labs envía la hora de Colombia marcada como UTC
// (la misma lectura llega 11:45:41Z por callback y 16:45:41Z por DataOS).
func (v *CallbackValidator) parseTimestamp(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}

	value = strings.TrimSuffix(value, "Z")

	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, true
	}

	t, err := time.ParseInLocation(naiveTimestampLayout, value, v.location)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
