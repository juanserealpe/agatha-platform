//go:build integration

// Tests de integración contra PostgreSQL real. Ejecutar con:
//
//	DATABASE_URL=postgres://agatha:agatha@127.0.0.1:5433/agatha?sslmode=disable \
//	  go test -tags integration ./internal/modules/baseline/infrastructure/persistence/repositories/
//
// Usan un measuring_point_id sintético y limpian sus filas al terminar.
package repositories

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

const testPoint int64 = 990000003

var t0 = time.Date(2026, 8, 6, 17, 0, 0, 0, time.UTC)

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

	if err := db.AutoMigrate(
		&models.BaselineVersionModel{},
		&models.BaselineComponentStatModel{},
		&models.ExclusionPeriodModel{},
	); err != nil {
		t.Fatalf("migración: %v", err)
	}

	cleanup := func() {
		db.Exec(`DELETE FROM baseline_component_stats WHERE baseline_version_id IN
			(SELECT id FROM baseline_versions WHERE measuring_point_id = ?)`, testPoint)
		db.Exec("DELETE FROM baseline_versions WHERE measuring_point_id = ?", testPoint)
		db.Exec("DELETE FROM baseline_exclusion_periods WHERE measuring_point_id = ?", testPoint)
	}
	cleanup()
	t.Cleanup(cleanup)

	return db
}

func version(validFrom time.Time, median float64) entities.BaselineVersion {
	return entities.BaselineVersion{
		MeasuringPointID: testPoint,
		ValidFrom:        validFrom,
		HistoryFrom:      validFrom.Add(-90 * 24 * time.Hour),
		HistoryTo:        validFrom,
		Signal:           entities.SignalBaseline{MedianB2: median, ThetaD: 67, DeltaMin: 1, NRecords: 100},
		Components: []entities.ComponentStat{
			{WindowHours: 24, Component: features.ComponentD, Median: 1, MAD: 0.1, NWindows: 10},
		},
		Confidence:       entities.ConfidenceNormal,
		Trigger:          entities.TriggerScheduled,
		AlgorithmVersion: features.AlgorithmVersion,
		CreatedAt:        validFrom,
	}
}

func countRows(t *testing.T, db *gorm.DB) (versions, stats int64) {
	t.Helper()
	db.Model(&models.BaselineVersionModel{}).Where("measuring_point_id = ?", testPoint).Count(&versions)
	db.Raw(`SELECT count(*) FROM baseline_component_stats WHERE baseline_version_id IN
		(SELECT id FROM baseline_versions WHERE measuring_point_id = ?)`, testPoint).Scan(&stats)
	return versions, stats
}

func TestReplaceHistory_ReplacesVersionsAndStats(t *testing.T) {
	db := openDB(t)
	repo := NewBaselineRepository(db)
	ctx := context.Background()

	week := 7 * 24 * time.Hour
	if _, err := repo.SaveHistory(ctx, testPoint, []entities.BaselineVersion{
		version(t0, 50), version(t0.Add(week), 51), version(t0.Add(2*week), 52),
	}); err != nil {
		t.Fatalf("SaveHistory: %v", err)
	}

	saved, deleted, err := repo.ReplaceHistory(ctx, testPoint, []entities.BaselineVersion{
		version(t0, 60), version(t0.Add(week), 61),
	})
	if err != nil {
		t.Fatalf("ReplaceHistory: %v", err)
	}

	if deleted != 3 || len(saved) != 2 || saved[0].Version != 1 || saved[1].Version != 2 {
		t.Fatalf("deleted=%d saved=%+v", deleted, saved)
	}
	if saved[0].ValidTo == nil || !saved[0].ValidTo.Equal(t0.Add(week)) || saved[1].ValidTo != nil {
		t.Fatal("la primera debe cerrar en la segunda y la última quedar vigente")
	}

	versions, stats := countRows(t, db)
	if versions != 2 || stats != 2 {
		t.Fatalf("filas: versiones=%d stats=%d, se esperaba 2 y 2", versions, stats)
	}

	latest, err := repo.GetLatest(ctx, testPoint)
	if err != nil || latest == nil || latest.Signal.MedianB2 != 61 {
		t.Fatalf("vigente = %+v err=%v", latest, err)
	}
}

func TestReplaceHistory_OnAPointWithoutVersionsBehavesLikeSaveHistory(t *testing.T) {
	db := openDB(t)

	saved, deleted, err := NewBaselineRepository(db).ReplaceHistory(context.Background(), testPoint,
		[]entities.BaselineVersion{version(t0, 60)})
	if err != nil || deleted != 0 || len(saved) != 1 {
		t.Fatalf("saved=%+v deleted=%d err=%v", saved, deleted, err)
	}
}

func TestReplaceHistory_InvalidHistoryKeepsTheOldOne(t *testing.T) {
	db := openDB(t)
	repo := NewBaselineRepository(db)
	ctx := context.Background()

	if _, err := repo.SaveHistory(ctx, testPoint, []entities.BaselineVersion{version(t0, 50)}); err != nil {
		t.Fatalf("SaveHistory: %v", err)
	}

	// valid_from no creciente: se rechaza antes de la TX.
	if _, _, err := repo.ReplaceHistory(ctx, testPoint, []entities.BaselineVersion{
		version(t0, 60), version(t0, 61),
	}); err == nil {
		t.Fatal("se esperaba error por valid_from no creciente")
	}

	versions, stats := countRows(t, db)
	if versions != 1 || stats != 1 {
		t.Fatalf("el histórico anterior debía quedar intacto: versiones=%d stats=%d", versions, stats)
	}
}

func alertPeriod(code string, started time.Time, ended *time.Time) entities.ExclusionPeriod {
	return entities.ExclusionPeriod{
		MeasuringPointID: testPoint,
		StartedAt:        started,
		EndedAt:          ended,
		Source:           entities.ExclusionSourceAlert,
		SourceAlertCode:  code,
	}
}

func TestExclusionPeriods_UpsertAlertIsIdempotentByCode(t *testing.T) {
	db := openDB(t)
	repo := NewExclusionPeriodRepository(db)
	ctx := context.Background()
	code := "ALR-TEST-990000003"
	t.Cleanup(func() { db.Exec("DELETE FROM baseline_exclusion_periods WHERE source_alert_code = ?", code) })

	// OPENED: en curso.
	if err := repo.UpsertAlert(ctx, alertPeriod(code, t0, nil)); err != nil {
		t.Fatalf("upsert abierto: %v", err)
	}
	// FINISHED: misma alerta, ahora con fin.
	end := t0.Add(30 * time.Hour)
	if err := repo.UpsertAlert(ctx, alertPeriod(code, t0, &end)); err != nil {
		t.Fatalf("upsert cerrado: %v", err)
	}

	var rows []models.ExclusionPeriodModel
	db.Where("source_alert_code = ?", code).Find(&rows)
	if len(rows) != 1 || rows[0].EndedAt == nil || !rows[0].EndedAt.Equal(end) || rows[0].Source != "ALERT" {
		t.Fatalf("filas = %+v", rows)
	}

	// REACTIVATED: vuelve a estar en curso.
	if err := repo.UpsertAlert(ctx, alertPeriod(code, t0, nil)); err != nil {
		t.Fatalf("upsert reactivado: %v", err)
	}
	db.Where("source_alert_code = ?", code).Find(&rows)
	if len(rows) != 1 || rows[0].EndedAt != nil {
		t.Fatalf("reactivada debía quedar en curso: %+v", rows)
	}
}

func TestExclusionPeriods_GetKnownEventsOverlapsTheRange(t *testing.T) {
	db := openDB(t)
	repo := NewExclusionPeriodRepository(db)
	ctx := context.Background()
	t.Cleanup(func() { db.Exec("DELETE FROM baseline_exclusion_periods WHERE source_alert_code LIKE 'ALR-TEST-K%'") })

	h := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Hour) }
	end := func(n int) *time.Time { v := h(n); return &v }
	for _, p := range []entities.ExclusionPeriod{
		alertPeriod("ALR-TEST-K1", h(0), end(5)),   // antes del rango
		alertPeriod("ALR-TEST-K2", h(8), end(12)),  // se solapa
		alertPeriod("ALR-TEST-K3", h(15), nil),     // en curso ⇒ hasta to
		alertPeriod("ALR-TEST-K4", h(40), end(45)), // después del rango
	} {
		if err := repo.UpsertAlert(ctx, p); err != nil {
			t.Fatalf("upsert %s: %v", p.SourceAlertCode, err)
		}
	}

	got, err := repo.GetKnownEvents(ctx, testPoint, h(10), h(20))
	if err != nil {
		t.Fatalf("GetKnownEvents: %v", err)
	}
	if len(got) != 2 || !got[0].From.Equal(h(8)) || !got[0].To.Equal(h(12)) ||
		!got[1].From.Equal(h(15)) || !got[1].To.Equal(h(20)) {
		t.Fatalf("eventos = %+v", got)
	}
}

func TestExclusionPeriods_ChecksRejectInconsistentRows(t *testing.T) {
	db := openDB(t)

	// ALERT sin código.
	err := db.Exec(`INSERT INTO baseline_exclusion_periods (measuring_point_id, started_at, source, note, created_at, updated_at)
		VALUES (?, ?, 'ALERT', '', now(), now())`, testPoint, t0).Error
	if err == nil {
		t.Fatal("ALERT sin source_alert_code debía violar el CHECK")
	}

	// fin antes del inicio.
	err = db.Exec(`INSERT INTO baseline_exclusion_periods (measuring_point_id, started_at, ended_at, source, note, created_at, updated_at)
		VALUES (?, ?, ?, 'MANUAL', 'x', now(), now())`, testPoint, t0, t0.Add(-time.Hour)).Error
	if err == nil {
		t.Fatal("ended_at < started_at debía violar el CHECK")
	}

	// MANUAL válido.
	if err := db.Exec(`INSERT INTO baseline_exclusion_periods (measuring_point_id, started_at, ended_at, source, note, created_at, updated_at)
		VALUES (?, ?, ?, 'MANUAL', 'mantenimiento', now(), now())`, testPoint, t0, t0.Add(time.Hour)).Error; err != nil {
		t.Fatalf("MANUAL válido rechazado: %v", err)
	}
}

func TestAlertIntervalReader_ReadsEveryAlert(t *testing.T) {
	db := openDB(t)
	if !db.Migrator().HasTable("alerts") {
		t.Skip("no existe la tabla alerts")
	}

	var want int64
	db.Raw("SELECT count(*) FROM alerts").Scan(&want)

	got, err := NewAlertIntervalReader(db).ListAlertIntervals(context.Background())
	if err != nil {
		t.Fatalf("ListAlertIntervals: %v", err)
	}
	if int64(len(got)) != want {
		t.Fatalf("leídas %d, alerts tiene %d", len(got), want)
	}
	for _, p := range got {
		if p.SourceAlertCode == "" || p.MeasuringPointID <= 0 || p.StartedAt.IsZero() || p.Source != entities.ExclusionSourceAlert {
			t.Fatalf("período inválido: %+v", p)
		}
	}
}

func TestExclusionPeriods_CountOpenAlertsIgnoresClosedAndManual(t *testing.T) {
	db := openDB(t)
	repo := NewExclusionPeriodRepository(db)
	ctx := context.Background()
	t.Cleanup(func() { db.Exec("DELETE FROM baseline_exclusion_periods WHERE source_alert_code LIKE 'ALR-TEST-O%'") })

	if n, err := repo.CountOpenAlerts(ctx, testPoint); err != nil || n != 0 {
		t.Fatalf("punto vacío: n=%d err=%v", n, err)
	}

	end := t0.Add(5 * time.Hour)
	for _, p := range []entities.ExclusionPeriod{
		alertPeriod("ALR-TEST-O1", t0, &end), // cerrada
		alertPeriod("ALR-TEST-O2", t0, nil),  // vigente
	} {
		if err := repo.UpsertAlert(ctx, p); err != nil {
			t.Fatalf("upsert %s: %v", p.SourceAlertCode, err)
		}
	}
	// MANUAL abierta: el rebuild no la toca, no cuenta.
	if err := db.Exec(`INSERT INTO baseline_exclusion_periods (measuring_point_id, started_at, source, note, created_at, updated_at)
		VALUES (?, ?, 'MANUAL', 'obra', now(), now())`, testPoint, t0).Error; err != nil {
		t.Fatal(err)
	}

	if n, err := repo.CountOpenAlerts(ctx, testPoint); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v, se esperaba 1", n, err)
	}
	if n, _ := repo.CountOpenAlerts(ctx, testPoint+1); n != 0 {
		t.Fatalf("otro punto: n=%d", n)
	}
}
