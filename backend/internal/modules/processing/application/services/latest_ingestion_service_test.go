package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type fakeMeasurementRepository struct {
	measurements []entities.RawMeasurement
	err          error
	calls        int
}

func (f *fakeMeasurementRepository) GetLatestMeasurements(_ context.Context, _ int64,
) ([]entities.RawMeasurement, error) {
	f.calls++
	return f.measurements, f.err
}

func newTestService(repo *fakeMeasurementRepository) *LatestIngestionService {
	return NewLatestIngestionService(repo, logger.New("ProcessingTest", "Info"))
}

func TestGetLatestIngestion(t *testing.T) {
	timestamp := time.Date(2026, 10, 2, 15, 0, 0, 0, time.UTC)
	measurements := []entities.RawMeasurement{
		{Timestamp: timestamp, MeasuringPointID: 7, Value: 21.5, Metric: entities.MetricDefinition{ID: 1, Key: "temperature"}},
		{Timestamp: timestamp, MeasuringPointID: 7, Value: 80, Metric: entities.MetricDefinition{ID: 2, Key: "humidity"}},
	}

	tests := []struct {
		name             string
		measuringPointID int64
		repo             *fakeMeasurementRepository
		wantErr          error
		wantCalls        int
		wantVariables    int
	}{
		{
			name:             "devuelve las variables de la última fecha",
			measuringPointID: 7,
			repo:             &fakeMeasurementRepository{measurements: measurements},
			wantCalls:        1,
			wantVariables:    2,
		},
		{
			name:             "rechaza un punto de medición inválido sin consultar la BD",
			measuringPointID: 0,
			repo:             &fakeMeasurementRepository{},
			wantErr:          domain.ErrInvalidMeasuringPointID,
			wantCalls:        0,
		},
		{
			name:             "traduce el error de BD a un AppError",
			measuringPointID: 7,
			repo:             &fakeMeasurementRepository{err: errors.New("connection refused")},
			wantErr:          domain.ErrConsultingLatestMeasurements,
			wantCalls:        1,
		},
		{
			name:             "sin mediciones devuelve not found",
			measuringPointID: 7,
			repo:             &fakeMeasurementRepository{measurements: []entities.RawMeasurement{}},
			wantErr:          domain.ErrLatestMeasurementsNotFound,
			wantCalls:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := newTestService(tt.repo).GetLatestIngestion(context.Background(), tt.measuringPointID)

			if tt.repo.calls != tt.wantCalls {
				t.Fatalf("llamadas al repositorio = %d, se esperaban %d", tt.repo.calls, tt.wantCalls)
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, se esperaba %v", err, tt.wantErr)
				}
				if result != nil {
					t.Fatalf("resultado = %+v, se esperaba nil", result)
				}
				return
			}

			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if result.MeasuringPointID != tt.measuringPointID {
				t.Errorf("measuring_point_id = %d, se esperaba %d", result.MeasuringPointID, tt.measuringPointID)
			}
			if !result.Timestamp.Equal(timestamp) {
				t.Errorf("timestamp = %v, se esperaba %v", result.Timestamp, timestamp)
			}
			if len(result.Measurements) != tt.wantVariables {
				t.Errorf("variables = %d, se esperaban %d", len(result.Measurements), tt.wantVariables)
			}
		})
	}
}
