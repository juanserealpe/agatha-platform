//go:build integration

// Tests de integración contra PostgreSQL real (15.4). Ejecutar con:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/modules/baseline/infrastructure/persistence/repositories/
//
// Usan un measuring_point_id sintético y limpian sus filas al terminar.
package repositories

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

const testPoint int64 = 990000001

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

	if err := db.AutoMigrate(&models.BaselineVersionModel{}, &models.BaselineComponentStatModel{}); err != nil {
		t.Fatalf("migración: %v", err)
	}

	cleanup := func() {
		db.Exec(`DELETE FROM baseline_component_stats WHERE baseline_version_id IN
			(SELECT id FROM baseline_versions WHERE measuring_point_id = ?)`, testPoint)
		db.Exec("DELETE FROM baseline_versions WHERE measuring_point_id = ?", testPoint)
		db.Exec("DELETE FROM raw_measurements WHERE measuring_point_id = ?", testPoint)
	}
	cleanup()
	t.Cleanup(cleanup)

	return db
}

func version(validFrom time.Time) entities.BaselineVersion {
	return entities.BaselineVersion{
		MeasuringPointID: testPoint,
		ValidFrom:        validFrom,
		HistoryFrom:      validFrom.AddDate(0, 0, -90),
		HistoryTo:        validFrom,
		Signal:           entities.SignalBaseline{MedianB2: 54, ThetaD: 67, DeltaMin: 1, NRecords: 8000},
		Components:       []entities.ComponentStat{{WindowHours: 24, Component: "C", Median: 0.2, MAD: 0.05, NWindows: 300}},
		Confidence:       entities.ConfidenceNormal,
		ExcludedPeriods:  []entities.ExcludedPeriod{},
		Flags:            []entities.Flag{entities.FlagSignalLowVariability},
		Trigger:          entities.TriggerManual,
		AlgorithmVersion: "1.0.0",
		CreatedAt:        validFrom,
	}
}

func TestMigrationCreatesPartialIndex(t *testing.T) {
	db := openDB(t)

	var def string
	if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE indexname = 'ux_baseline_versions_current'`).
		Scan(&def).Error; err != nil || def == "" {
		t.Fatalf("no existe el índice parcial (err %v)", err)
	}
	t.Logf("índice: %s", def)
}

func TestVersioning(t *testing.T) {
	db := openDB(t)
	repo := NewBaselineRepository(db)
	ctx := context.Background()
	t1 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.AddDate(0, 0, 7)

	v1, err := repo.SaveNewVersion(ctx, version(t1))
	if err != nil || v1.Version != 1 {
		t.Fatalf("v1 = %+v, err = %v", v1, err)
	}
	v2, err := repo.SaveNewVersion(ctx, version(t2))
	if err != nil || v2.Version != 2 {
		t.Fatalf("v2 = %+v, err = %v", v2, err)
	}

	cases := []struct {
		at   time.Time
		want int // 0 = ninguna
	}{
		{t1.Add(-time.Second), 0},
		{t1, 1},
		{t2.Add(-time.Second), 1},
		{t2, 2},
		{t2.AddDate(1, 0, 0), 2},
	}
	for _, c := range cases {
		got, err := repo.GetEffective(ctx, testPoint, c.at)
		if err != nil {
			t.Fatal(err)
		}
		if (c.want == 0) != (got == nil) || (got != nil && got.Version != c.want) {
			t.Fatalf("GetEffective(%v) = %+v, se esperaba versión %d", c.at, got, c.want)
		}
	}

	got, _ := repo.GetEffective(ctx, testPoint, t1)
	if got.ValidTo == nil || !got.ValidTo.Equal(t2) || len(got.Components) != 1 || got.Flags[0] != entities.FlagSignalLowVariability {
		t.Fatalf("v1 tras cierre = %+v", got)
	}

	latest, _ := repo.GetLatest(ctx, testPoint)
	if latest.Version != 2 || latest.ValidTo != nil {
		t.Fatalf("latest = %+v", latest)
	}
}

func TestConcurrentInsertsKeepOneCurrent(t *testing.T) {
	db := openDB(t)
	repo := NewBaselineRepository(db)
	ctx := context.Background()
	t1 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = repo.SaveNewVersion(ctx, version(t1.Add(time.Duration(i+1)*time.Hour)))
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil && !errors.Is(err, domain.ErrBaselineConcurrentUpdate) && err.Error() != errNonMonotonicValidFrom.Error() {
			t.Fatalf("error inesperado: %v", err)
		}
	}

	var current int64
	db.Model(&models.BaselineVersionModel{}).
		Where("measuring_point_id = ? AND valid_to IS NULL", testPoint).
		Count(&current)
	if current != 1 {
		t.Fatalf("versiones vigentes = %d, se esperaba 1", current)
	}
}

func TestGetSamplesPivot(t *testing.T) {
	db := openDB(t)
	reader := NewHistoryReader(db, mappers.MetricIDs{B1: 12, B2: 13, Planitud: 18})
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(15 * time.Minute)

	insert := func(ts time.Time, metric uint64, value float64) {
		if err := db.Exec(`INSERT INTO raw_measurements (measuring_point_id, metric_id, timestamp, value, source)
			VALUES (?, ?, ?, ?, 'test')`, testPoint, metric, ts, value).Error; err != nil {
			t.Fatal(err)
		}
	}
	insert(t0, 12, 1)
	insert(t0, 13, 50)
	insert(t0, 18, 0.4)
	insert(t1, 13, 51) // faltan B1 y Planitud
	insert(t1, 5, 99)  // métrica ajena

	samples, err := reader.GetSamples(ctx, testPoint, t0, t0.Add(time.Hour))
	if err != nil || len(samples) != 2 {
		t.Fatalf("samples = %+v, err = %v", samples, err)
	}
	if !samples[0].HasB1 || !samples[0].HasPl || samples[1].HasB1 || samples[1].HasPl || samples[1].B2 != 51 {
		t.Fatalf("pivote incorrecto: %+v", samples)
	}

	first, err := reader.GetFirstSeenAt(ctx, testPoint)
	if err != nil || first == nil || !first.Equal(t0) {
		t.Fatalf("first = %v, err = %v", first, err)
	}
}

func TestSaveHistory(t *testing.T) {
	db := openDB(t)
	repo := NewBaselineRepository(db)
	ctx := context.Background()
	t1 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.AddDate(0, 0, 1)
	t3 := t2.AddDate(0, 0, 7)

	// Fuera de orden: no escribe nada.
	if _, err := repo.SaveHistory(ctx, testPoint, []entities.BaselineVersion{version(t2), version(t1)}); err == nil {
		t.Fatal("se esperaba error por valid_from no creciente")
	}
	if latest, _ := repo.GetLatest(ctx, testPoint); latest != nil {
		t.Fatalf("un histórico inválido dejó versiones: %+v", latest)
	}

	saved, err := repo.SaveHistory(ctx, testPoint, []entities.BaselineVersion{version(t1), version(t2), version(t3)})
	if err != nil || len(saved) != 3 {
		t.Fatalf("guardadas = %d, err = %v", len(saved), err)
	}
	for i, v := range saved {
		if v.ID == 0 || v.Version != i+1 || len(v.Components) != 1 {
			t.Fatalf("versión %d = %+v", i, v)
		}
	}

	cases := []struct {
		at   time.Time
		want int // 0 = ninguna
	}{
		{t1.Add(-time.Second), 0},
		{t1, 1},
		{t2, 2},
		{t3.Add(-time.Second), 2},
		{t3.AddDate(1, 0, 0), 3},
	}
	for _, c := range cases {
		got, err := repo.GetEffective(ctx, testPoint, c.at)
		if err != nil {
			t.Fatal(err)
		}
		if (c.want == 0) != (got == nil) || (got != nil && got.Version != c.want) {
			t.Fatalf("GetEffective(%v) = %+v, se esperaba versión %d", c.at, got, c.want)
		}
	}

	if latest, _ := repo.GetLatest(ctx, testPoint); latest.Version != 3 || latest.ValidTo != nil {
		t.Fatalf("latest = %+v", latest)
	}

	// Con versiones existentes no se puede volver a cargar el histórico.
	if _, err := repo.SaveHistory(ctx, testPoint, []entities.BaselineVersion{version(t3.AddDate(0, 0, 7))}); !errors.Is(err, domain.ErrBaselineHistoryExists) {
		t.Fatalf("err = %v, se esperaba ErrBaselineHistoryExists", err)
	}

	// El versionado normal continúa desde la última versión del histórico.
	v4, err := repo.SaveNewVersion(ctx, version(t3.AddDate(0, 0, 7)))
	if err != nil || v4.Version != 4 {
		t.Fatalf("v4 = %+v, err = %v", v4, err)
	}
}
