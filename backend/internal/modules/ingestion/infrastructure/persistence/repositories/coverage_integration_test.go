//go:build integration

package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestCoverage_LatestAndDistinctTimestamps(t *testing.T) {
	db := openDB(t)
	if err := db.AutoMigrate(&models.RawMeasurementModel{}, &models.IngestionStateModel{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	coverage := NewMeasurementCoverageRepository(db)

	latest, err := coverage.LatestTimestamp(ctx, testPoint)
	if err != nil || latest != nil {
		t.Fatalf("punto sin datos: latest = %v, err = %v", latest, err)
	}

	// Dos métricas por instante: ListTimestamps devuelve cada instante una vez.
	var rows []entities.RawMeasurement
	for i := 0; i < 3; i++ {
		at := t0.Add(time.Duration(i) * 15 * time.Minute)
		rows = append(rows,
			entities.RawMeasurement{Timestamp: at, MeasuringPointID: testPoint, MetricID: 12, Value: 1, Source: "callback"},
			entities.RawMeasurement{Timestamp: at, MeasuringPointID: testPoint, MetricID: 13, Value: 2, Source: "callback"},
		)
	}
	if _, err := NewMeasurementRepository(db, logger.New("test", "Info")).SaveBatch(ctx, rows); err != nil {
		t.Fatal(err)
	}

	latest, err = coverage.LatestTimestamp(ctx, testPoint)
	if err != nil || latest == nil || !latest.Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("latest = %v, err = %v", latest, err)
	}

	got, err := coverage.ListTimestamps(ctx, testPoint, t0, t0.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Equal(t0) || !got[1].Equal(t0.Add(15*time.Minute)) {
		t.Fatalf("timestamps = %v, se esperaban t0 y t0+15m (bordes incluidos, sin repetir)", got)
	}
}
