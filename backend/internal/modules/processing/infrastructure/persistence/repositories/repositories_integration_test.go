//go:build integration

// Tests de integración contra PostgreSQL real. Ejecutar con:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/modules/processing/infrastructure/persistence/repositories/
//
// Usan un measuring_point_id sintético y limpian sus filas al terminar.
// raw_measurements debe existir (la crea Ingestion).
package repositories

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

const testPoint int64 = 990000002

var t0 = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

func openDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no configurada")
	}

	db, err := database.NewPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.AutoMigrate(&models.FeatureEvaluationModel{}); err != nil {
		t.Fatalf("migración: %v", err)
	}

	cleanup := func() {
		db.Exec("DELETE FROM feature_evaluations WHERE measuring_point_id = ?", testPoint)
		db.Exec("DELETE FROM raw_measurements WHERE measuring_point_id = ?", testPoint)
	}
	cleanup()
	t.Cleanup(cleanup)

	return db
}

func evaluation(at time.Time, windowHours int) entities.FeatureEvaluation {
	return entities.FeatureEvaluation{
		MeasuringPointID: testPoint,
		WindowHours:      windowHours,
		EvaluatedAt:      at,
		WindowStart:      at.Add(-time.Duration(windowHours) * time.Hour),
		Status:           entities.StatusNoCalculable,
		NoCalcReason:     entities.ReasonLowCoverage,
		NExpected:        96,
		Flags:            []string{"CONSTANT_SERIES_B1"},
		AlgorithmVersion: "1.0.0",
		ConfigVersion:    "m=12,13,18",
		Trigger:          entities.TriggerReconciler,
		CreatedAt:        time.Now().UTC(),
	}
}

func TestUniqueIndexExists(t *testing.T) {
	db := openDB(t)

	var count int64
	db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'feature_evaluations'
		AND indexname = 'ux_feature_evaluations_point_window_hour' AND indexdef LIKE 'CREATE UNIQUE%'`).Scan(&count)
	if count != 1 {
		t.Fatal("falta el índice único ux_feature_evaluations_point_window_hour")
	}
}

func TestSaveAllIsIdempotentAndLastEvaluatedAtUses24h(t *testing.T) {
	db := openDB(t)
	repo := NewEvaluationRepository(db)
	ctx := context.Background()

	if last, err := repo.GetLastEvaluatedAt(ctx, testPoint); err != nil || last != nil {
		t.Fatalf("sin filas: last = %v, err = %v", last, err)
	}

	batch := []entities.FeatureEvaluation{
		evaluation(t0, 24), evaluation(t0, 6),
		evaluation(t0.Add(time.Hour), 24), evaluation(t0.Add(time.Hour), 6),
		evaluation(t0.Add(2*time.Hour), 6), // solo 6 h: no cuenta como última hora
	}

	inserted, err := repo.SaveAll(ctx, batch)
	if err != nil || inserted != 5 {
		t.Fatalf("primer SaveAll: inserted = %d, err = %v", inserted, err)
	}

	inserted, err = repo.SaveAll(ctx, batch[:2])
	if err != nil || inserted != 0 {
		t.Fatalf("SaveAll repetido: inserted = %d, err = %v (se esperaba 0 sin error)", inserted, err)
	}

	last, err := repo.GetLastEvaluatedAt(ctx, testPoint)
	if err != nil || last == nil || !last.Equal(t0.Add(time.Hour)) {
		t.Fatalf("last = %v, err = %v", last, err)
	}

	var stored models.FeatureEvaluationModel
	db.Where("measuring_point_id = ? AND window_hours = 24 AND evaluated_at = ?", testPoint, t0).First(&stored)
	got := mappers.ToFeatureEvaluation(stored)
	if got.NoCalcReason != entities.ReasonLowCoverage || len(got.Flags) != 1 || got.I != nil {
		t.Fatalf("fila guardada = %+v", got)
	}
}

func TestSaveAllIsAtomic(t *testing.T) {
	db := openDB(t)
	repo := NewEvaluationRepository(db)

	bad := evaluation(t0.Add(time.Hour), 24)
	bad.WindowHours = 12 // viola el CHECK

	if _, err := repo.SaveAll(context.Background(), []entities.FeatureEvaluation{evaluation(t0, 24), bad}); err == nil {
		t.Fatal("se esperaba error por el CHECK de window_hours")
	}

	var count int64
	db.Model(&models.FeatureEvaluationModel{}).Where("measuring_point_id = ?", testPoint).Count(&count)
	if count != 0 {
		t.Fatalf("filas = %d: la TX debía revertir todo", count)
	}
}

func TestMeasurementReader(t *testing.T) {
	db := openDB(t)
	reader := NewMeasurementReader(db, mappers.MetricIDs{B1: 12, B2: 13, Planitud: 18})
	ctx := context.Background()

	if latest, err := reader.GetLatestTimestamp(ctx, testPoint); err != nil || latest != nil {
		t.Fatalf("sin datos: latest = %v, err = %v", latest, err)
	}

	for i, metric := range []int{12, 13, 18, 13} {
		ts := t0.Add(time.Duration(i/3) * 15 * time.Minute)
		if err := db.Exec(`INSERT INTO raw_measurements (measuring_point_id, metric_id, timestamp, value, source)
			VALUES (?, ?, ?, ?, 'test')`, testPoint, metric, ts, float64(i)).Error; err != nil {
			t.Fatalf("insertar medición: %v", err)
		}
	}

	samples, err := reader.GetSamples(ctx, testPoint, t0, t0.Add(time.Hour))
	if err != nil || len(samples) != 2 || !samples[0].HasB1 || !samples[0].HasPl || !samples[1].HasB2 {
		t.Fatalf("samples = %+v, err = %v", samples, err)
	}

	latest, err := reader.GetLatestTimestamp(ctx, testPoint)
	if err != nil || latest == nil || !latest.Equal(t0.Add(15*time.Minute)) {
		t.Fatalf("latest = %v, err = %v", latest, err)
	}
}
