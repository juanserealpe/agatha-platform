package repositories

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// captureInsertSQL ejecuta SaveBatch en DryRun (sin BD) y devuelve el INSERT generado.
func captureInsertSQL(t *testing.T, measurements []entities.RawMeasurement) string {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost dbname=dryrun sslmode=disable"}),
		// En producción SaveBatch corre dentro de la TX del UnitOfWork; acá se
		// omite la TX por defecto para que DryRun no abra una conexión.
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}

	var statements []string
	if err := db.Callback().Create().After("gorm:create").Register("test:capture", func(tx *gorm.DB) {
		statements = append(statements, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := NewMeasurementRepository(db, logger.New("test", "Info")).SaveBatch(context.Background(), measurements); err != nil {
		t.Fatal(err)
	}
	if len(statements) == 0 {
		t.Fatal("SaveBatch no generó ningún INSERT")
	}
	return statements[0]
}

func TestSaveBatch_IgnoresDuplicatesAndLinksTheBatch(t *testing.T) {
	batchID := int64(42)
	sql := captureInsertSQL(t, []entities.RawMeasurement{{
		Timestamp:        time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC),
		MeasuringPointID: 11556,
		MetricID:         12,
		Value:            50,
		Source:           "dataos",
		IngestionBatchID: &batchID,
	}})
	t.Log(sql)

	if !strings.Contains(sql, `ON CONFLICT ("measuring_point_id","metric_id","timestamp") DO NOTHING`) {
		t.Fatalf("falta ON CONFLICT ... DO NOTHING sobre la clave natural:\n%s", sql)
	}
	if !strings.Contains(sql, `"ingestion_batch_id"`) {
		t.Fatalf("el INSERT no escribe ingestion_batch_id:\n%s", sql)
	}
	// id es GENERATED ALWAYS: enviarlo hace fallar el INSERT.
	columns := sql[:strings.Index(sql, "VALUES")]
	if strings.Contains(columns, `"id"`) {
		t.Fatalf("el INSERT no debe enviar id:\n%s", sql)
	}
}
