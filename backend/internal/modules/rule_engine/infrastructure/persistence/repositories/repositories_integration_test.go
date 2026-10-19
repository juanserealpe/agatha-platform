//go:build integration

// Tests de integración contra PostgreSQL real. Ejecutar con:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/modules/rule_engine/infrastructure/persistence/repositories/
//
// Migran el esquema del Rule Engine (incluido btree_gist + EXCLUDE), usan un
// measuring_point_id sintético y limpian sus filas al terminar.
package repositories

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/database"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const testPoint int64 = 990000003

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
	if err := persistence.Migrate(db); err != nil {
		t.Fatalf("migración: %v", err)
	}

	cleanup := func() {
		db.Exec("DELETE FROM rule_evaluations WHERE measuring_point_id = ?", testPoint)
		db.Exec("DELETE FROM rule_threshold_versions WHERE measuring_point_id = ?", testPoint)
	}
	cleanup()
	t.Cleanup(cleanup)

	return db
}

func version(n int, from time.Time, to *time.Time) entities.ThresholdVersion {
	return entities.ThresholdVersion{
		MeasuringPointID: testPoint,
		EventType:        entities.EventTypeLeak,
		Version:          n,
		ThresholdParams:  entities.DefaultThresholdParams(),
		ValidFrom:        from,
		ValidTo:          to,
		Reason:           "test",
		CreatedBy:        "test",
		CreatedAt:        from,
	}
}

func insert(t *testing.T, repo *ThresholdRepository, v entities.ThresholdVersion) (int64, error) {
	t.Helper()
	var id int64
	err := repo.Transaction(context.Background(), func(tx domainRepositories.ThresholdWriter) error {
		var err error
		id, err = tx.Insert(context.Background(), v)
		return err
	})
	return id, err
}

func TestSchemaHasExcludeAndChecks(t *testing.T) {
	db := openDB(t)

	var count int64
	db.Raw(`SELECT COUNT(*) FROM pg_constraint WHERE conname = ? AND contype = 'x'`, persistence.ExcludeConstraint).Scan(&count)
	if count != 1 {
		t.Fatalf("falta el EXCLUDE %s", persistence.ExcludeConstraint)
	}

	db.Raw(`SELECT COUNT(*) FROM pg_constraint WHERE conrelid = 'rule_threshold_versions'::regclass AND contype = 'c'`).Scan(&count)
	if count < 6 {
		t.Fatalf("CHECKs de rule_threshold_versions = %d, se esperaban 6", count)
	}

	// Migrate es idempotente.
	if err := persistence.Migrate(db); err != nil {
		t.Fatalf("segunda migración: %v", err)
	}
}

func TestExcludeRejectsOverlappingVersions(t *testing.T) {
	db := openDB(t)
	repo := NewThresholdRepository(db)

	if _, err := insert(t, repo, version(1, t0, nil)); err != nil {
		t.Fatal(err)
	}

	// Versión 2 que empieza dentro de la vigencia abierta de la v1.
	if _, err := insert(t, repo, version(2, t0.Add(time.Hour), nil)); err == nil {
		t.Fatal("el EXCLUDE debía rechazar la versión solapada")
	}
}

func TestChecksRejectInvalidThresholds(t *testing.T) {
	db := openDB(t)
	v := version(1, t0, nil)
	v.Naranja = v.Amarillo // viola amarillo < naranja

	if _, err := insert(t, NewThresholdRepository(db), v); err == nil {
		t.Fatal("el CHECK de orden debía rechazar la versión")
	}
}

func TestCreateVersionClosesCurrentInTheSameTransaction(t *testing.T) {
	db := openDB(t)
	repo := NewThresholdRepository(db)
	ctx := context.Background()
	if _, err := insert(t, repo, version(1, t0, nil)); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := services.NewThresholdService(repo, ports.ClockFunc(func() time.Time { return now }), logger.New("Test", "Error"))

	created, err := svc.CreateThresholdVersion(ctx, services.CreateThresholdVersionCommand{
		PointID: testPoint, Patch: entities.ThresholdPatch{Rojo: entities.Some(0.40)}, Reason: "r", CreatedBy: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	history, _ := repo.GetHistory(ctx, testPoint, entities.EventTypeLeak)
	if len(history) != 2 || history[0].ValidTo == nil || !history[0].ValidTo.Equal(now) ||
		history[1].ID != created.ID || history[1].Rojo != 0.40 || history[1].ValidTo != nil {
		t.Fatalf("historial = %+v", history)
	}

	// Una TX que falla después de cerrar no deja la vigente cerrada.
	boom := errors.New("boom")
	err = repo.Transaction(ctx, func(tx domainRepositories.ThresholdWriter) error {
		open, err := tx.LockOpen(ctx, testPoint, entities.EventTypeLeak)
		if err != nil {
			return err
		}
		if err := tx.Close(ctx, open.ID, now.Add(time.Hour)); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	current, _ := repo.GetEffectiveAt(ctx, testPoint, entities.EventTypeLeak, now.Add(2*time.Hour))
	if current == nil || current.ID != created.ID || current.ValidTo != nil {
		t.Fatalf("el rollback debía dejar abierta la v2: %+v", current)
	}

	// Validación: fecha pasada no escribe nada.
	if _, err := svc.CreateThresholdVersion(ctx, services.CreateThresholdVersionCommand{
		PointID: testPoint, ValidFrom: now.Add(-time.Hour), Reason: "r", CreatedBy: "c",
	}); !errors.Is(err, domain.ErrValidFromInPast) {
		t.Fatalf("err = %v", err)
	}
}

func TestTimelineAndEffectiveAt(t *testing.T) {
	db := openDB(t)
	repo := NewThresholdRepository(db)
	ctx := context.Background()

	d1, d2 := t0.Add(24*time.Hour), t0.Add(48*time.Hour)
	for _, v := range []entities.ThresholdVersion{version(1, t0, &d1), version(2, d1, &d2), version(3, d2, nil)} {
		if _, err := insert(t, repo, v); err != nil {
			t.Fatal(err)
		}
	}

	at, _ := repo.GetEffectiveAt(ctx, testPoint, entities.EventTypeLeak, d1)
	if at == nil || at.Version != 2 {
		t.Fatalf("vigente en d1 = %+v (valid_to es exclusivo)", at)
	}

	overlapping, err := repo.GetOverlapping(ctx, testPoint, entities.EventTypeLeak, t0.Add(time.Hour), d2)
	if err != nil || len(overlapping) != 2 || overlapping[0].Version != 1 || overlapping[1].Version != 2 {
		t.Fatalf("solapadas = %+v, %v", overlapping, err)
	}
}

func TestRuleEvaluationSaveIsIdempotentAndPositionWorks(t *testing.T) {
	db := openDB(t)
	thresholds := NewThresholdRepository(db)
	evaluations := NewRuleEvaluationRepository(db)
	ctx := context.Background()

	versionID, err := insert(t, thresholds, version(1, t0, nil))
	if err != nil {
		t.Fatal(err)
	}

	row := entities.RuleEvaluation{
		MeasuringPointID: testPoint, EventType: entities.EventTypeLeak, EvaluatedAt: t0.Add(2 * time.Hour),
		IndicatorValue: 0.2034, RawLevel: entities.LevelAmarillo, Level: entities.LevelAmarillo,
		Rule: entities.RuleThreshold, InEpisode: true, HoursAbove: 2, HoursInLevel: 1,
		ThresholdVersionID: versionID, Emitted: entities.EmittedExceeded, CreatedAt: time.Now().UTC(),
	}

	id, inserted, err := evaluations.Save(ctx, row)
	if err != nil || !inserted || id == 0 {
		t.Fatalf("primer Save: id=%d inserted=%v err=%v", id, inserted, err)
	}
	if _, inserted, err := evaluations.Save(ctx, row); err != nil || inserted {
		t.Fatalf("Save repetido: inserted=%v err=%v (se esperaba false sin error)", inserted, err)
	}

	exists, later, err := evaluations.GetPosition(ctx, testPoint, entities.EventTypeLeak, t0.Add(2*time.Hour))
	if err != nil || !exists || later {
		t.Fatalf("posición en la hora: exists=%v later=%v err=%v", exists, later, err)
	}
	exists, later, _ = evaluations.GetPosition(ctx, testPoint, entities.EventTypeLeak, t0.Add(time.Hour))
	if exists || !later {
		t.Fatalf("posición anterior: exists=%v later=%v", exists, later)
	}

	previous, err := evaluations.GetPrevious(ctx, testPoint, entities.EventTypeLeak, t0.Add(3*time.Hour), 13)
	if err != nil || len(previous) != 1 || previous[0].IndicatorValue != 0.2034 || previous[0].Level != entities.LevelAmarillo {
		t.Fatalf("previas = %+v, %v", previous, err)
	}

	bad := row
	bad.EvaluatedAt, bad.Level = t0.Add(5*time.Hour), "VIOLETA"
	if _, _, err := evaluations.Save(ctx, bad); err == nil {
		t.Fatal("el CHECK de level debía rechazar la fila")
	}

	deleted, err := evaluations.DeleteRange(ctx, ptr(testPoint), entities.EventTypeLeak, t0, time.Time{})
	if err != nil || deleted != 1 {
		t.Fatalf("deleted = %d, err = %v", deleted, err)
	}
}

func ptr(v int64) *int64 { return &v }
