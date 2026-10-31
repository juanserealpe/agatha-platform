//go:build integration

// Tests de integración contra PostgreSQL real. Ejecutar con:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/modules/ingestion/infrastructure/persistence/repositories/
//
// Usan un measuring_point_id sintético y limpian sus filas al terminar.
// raw_measurements e ingestion_state_models deben existir (las crea bootstrap de Ingestion).
package repositories

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const testPoint int64 = 990000003

var t0 = time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

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

	cleanup := func() {
		db.Exec("DELETE FROM raw_measurements WHERE measuring_point_id = ?", testPoint)
		db.Exec("DELETE FROM ingestion_state_models WHERE measuring_point_id = ?", testPoint)
	}
	cleanup()
	t.Cleanup(cleanup)

	return db
}

func batch(at time.Time) []entities.RawMeasurement {
	return []entities.RawMeasurement{
		{Timestamp: at, MeasuringPointID: testPoint, MetricID: 12, Value: 50, Source: "dataos"},
		{Timestamp: at.Add(5 * time.Minute), MeasuringPointID: testPoint, MetricID: 12, Value: 51, Source: "dataos"},
	}
}

// ingest persiste el estado y luego el lote en una TX, como IngestService.
func ingest(ctx context.Context, uow *UnitOfWork, measurements []entities.RawMeasurement, failAfterState bool,
) (entities.SaveStateResult, entities.SaveBatchResult, error) {
	var (
		stateResult entities.SaveStateResult
		batchResult entities.SaveBatchResult
	)
	last := measurements[len(measurements)-1].Timestamp

	err := uow.Execute(ctx, func(ctx context.Context, m domainRepositories.MeasurementRepository,
		s domainRepositories.IngestionStateRepository,
	) error {
		saved, err := s.Save(ctx, &entities.IngestionState{
			MeasuringPointID: uint64(testPoint),
			Source:           "dataos",
			LastIngestedAt:   &last,
			ReceivedRecords:  len(measurements),
		})
		if err != nil {
			return err
		}
		for i := range measurements {
			measurements[i].IngestionBatchID = &saved.BatchSeq
		}
		batch, err := m.SaveBatch(ctx, measurements)
		if err != nil {
			return err
		}
		if failAfterState {
			return errors.New("falla simulada")
		}
		stateResult, batchResult = saved, batch
		return nil
	})
	return stateResult, batchResult, err
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	if err := db.Table(table).Where("measuring_point_id = ?", testPoint).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUnitOfWork_ConsecutiveBatchesAdvanceStateAndBatchSeq(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	uow := NewUnitOfWork(db, logger.New("ingestion-test", "INFO"))

	first, _, err := ingest(ctx, uow, batch(t0), false)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := ingest(ctx, uow, batch(t0.Add(15*time.Minute)), false)
	if err != nil {
		t.Fatal(err)
	}

	if second.BatchSeq <= first.BatchSeq {
		t.Fatalf("batch_seq no es creciente: %d → %d", first.BatchSeq, second.BatchSeq)
	}
	if !second.LastIngestedAt.After(first.LastIngestedAt) {
		t.Fatalf("last_ingested_at no avanzó: %v → %v", first.LastIngestedAt, second.LastIngestedAt)
	}

	latest, err := NewIngestionStateRepository(db).FindLatestByMeasuringPointID(ctx, testPoint)
	if err != nil || latest == nil {
		t.Fatalf("último estado = %v, error = %v", latest, err)
	}
	if int64(latest.ID) != second.BatchSeq || !latest.LastIngestedAt.Equal(second.LastIngestedAt) {
		t.Fatalf("último estado = %+v, se esperaba la fila %d", latest, second.BatchSeq)
	}
	if n := count(t, db, "ingestion_state_models"); n != 2 {
		t.Fatalf("filas en ingestion_state_models = %d, se esperaban 2", n)
	}
}

func TestUnitOfWork_RollbackLeavesNoMeasurementsWithoutState(t *testing.T) {
	db := openDB(t)

	if _, _, err := ingest(context.Background(), NewUnitOfWork(db, logger.New("ingestion-test", "INFO")), batch(t0), true); err == nil {
		t.Fatal("se esperaba error")
	}

	if m, s := count(t, db, "raw_measurements"), count(t, db, "ingestion_state_models"); m != 0 || s != 0 {
		t.Fatalf("tras el rollback quedaron %d mediciones y %d estados", m, s)
	}
}

func TestUnitOfWork_RepeatedMeasurementsAreSkippedAndLinkedToTheirBatch(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	uow := NewUnitOfWork(db, logger.New("ingestion-test", "INFO"))

	first, firstBatch, err := ingest(ctx, uow, batch(t0), false)
	if err != nil {
		t.Fatal(err)
	}
	if firstBatch.Inserted != 2 {
		t.Fatalf("primer lote: insertadas = %d, se esperaban 2", firstBatch.Inserted)
	}

	// Mismo punto, métrica e instante: el índice único descarta sin fallar.
	_, repeated, err := ingest(ctx, uow, batch(t0), false)
	if err != nil {
		t.Fatalf("un lote repetido no debe fallar: %v", err)
	}
	if repeated.Inserted != 0 {
		t.Fatalf("lote repetido: insertadas = %d, se esperaban 0", repeated.Inserted)
	}

	var linked int64
	if err := db.Table("raw_measurements").
		Where("measuring_point_id = ? AND ingestion_batch_id = ?", testPoint, first.BatchSeq).
		Count(&linked).Error; err != nil {
		t.Fatal(err)
	}
	if linked != 2 || count(t, db, "raw_measurements") != 2 {
		t.Fatalf("mediciones del lote %d = %d, se esperaban 2 y ninguna más", first.BatchSeq, linked)
	}
}
