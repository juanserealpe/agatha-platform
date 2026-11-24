package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
)

// firstMeasurement es la primera medición del punto A (anterior a day1 + horas del backfill).
var firstMeasurement = time.Date(2026, 7, 1, 12, 30, 0, 0, time.UTC)

// v2Params son los parámetros de la versión vigente: cortes de NARANJA y ROJO
// distintos de la v1 (referenceParams), para distinguir qué versión se aplicó.
func v2Params() entities.ThresholdParams {
	p := referenceParams()
	p.Naranja, p.Rojo = 0.30, 0.45
	return p
}

// resetFixture: el punto A con v1 (cerrada) y v2 (vigente, con cortes
// distintos) y el punto B con su v1; la v2 empieza en la segunda hora del backfill.
type resetFixture struct {
	*fixture
	resetter *memory.HistoryResetter
}

func newResetFixture() *resetFixture {
	v2From := hour(1)
	f := &resetFixture{fixture: newFixture(
		seedVersion(pointA, 1, day1, &v2From, referenceParams()),
		seedVersion(pointA, 2, v2From, nil, v2Params()),
		seedVersion(pointB, 1, day1, nil, referenceParams()),
	)}
	f.resetter = memory.NewHistoryResetter(f.thresholds, f.evaluations)
	f.resetter.FirstMeasurement = map[int64]time.Time{pointA: firstMeasurement}
	return f
}

func (f *resetFixture) backfill(env string) *services.BackfillService {
	return newBackfill(f.fixture, f.source, env, 100).WithHistoryResetter(f.resetter, fixedClock())
}

func resetCmd() services.BackfillCommand {
	return services.BackfillCommand{PointID: pointA, Rebuild: true, ResetVersions: true, ConfirmPointID: pointA}
}

func versionsOf(f *fixture, pointID int64) []entities.ThresholdVersion {
	var out []entities.ThresholdVersion
	for _, v := range f.thresholds.All() {
		if v.MeasuringPointID == pointID {
			out = append(out, v)
		}
	}
	return out
}

// I = 0,40: con la v1 es ROJO; con la v2 aplicada al histórico es NARANJA.
func feedRojoUnderV1(f *resetFixture) {
	for n, i := range []float64{0.40, 0.40, 0.40} {
		f.source.rows[pointA] = append(f.source.rows[pointA], ports.CalculatedIndicator{
			MeasuringPointID: pointA, EvaluatedAt: hour(n), I: i,
		})
	}
}

func TestBackfillReset_CollapsesVersionsAndRebuildsWithCurrentParams(t *testing.T) {
	f := newResetFixture()
	feedRojoUnderV1(f)
	// Filas previas de A (la primera hora con la v1: ROJO) y de B, que no se toca.
	f.source.add(pointB, 0.10, 0.10)
	if _, err := newBackfill(f.fixture, f.source, "development", 100).Backfill(context.Background(), services.BackfillCommand{}); err != nil {
		t.Fatal(err)
	}
	if before := f.evaluations.Rows(pointA); len(before) != 3 || before[0].ComputedLevel != entities.LevelRojo {
		t.Fatalf("antes del reset la hora 0 era ROJO con la v1: %+v", before)
	}
	rowsB := len(f.evaluations.Rows(pointB))

	result, err := f.backfill("development").Backfill(context.Background(), resetCmd())
	if err != nil {
		t.Fatal(err)
	}

	got := versionsOf(f.fixture, pointA)
	if len(got) != 1 {
		t.Fatalf("versiones del punto A = %+v; se esperaba una sola", got)
	}
	v := got[0]
	if v.Version != 1 || v.ValidTo != nil || !v.ValidFrom.Equal(firstMeasurement) || v.EventType != entities.EventTypeLeak ||
		v.CreatedBy != entities.SystemCreatedBy || v.Reason != "Reconstrucción: parámetros de la versión 2 aplicados al histórico" {
		t.Fatalf("versión resultante = %+v", v)
	}
	if v.Naranja != 0.30 || v.Rojo != 0.45 || v.Amarillo != 0.18 {
		t.Fatalf("los parámetros deben ser los de la v2: %+v", v.ThresholdParams)
	}
	if !v.CreatedAt.Equal(now) {
		t.Fatalf("created_at = %v, se esperaba %v", v.CreatedAt, now)
	}

	if result.VersionsCollapsed != 2 || result.ResetSourceVersion != 2 || result.ResetValidFrom == nil || !result.ResetValidFrom.Equal(firstMeasurement) {
		t.Fatalf("informe = %+v", result)
	}

	rows := f.evaluations.Rows(pointA)
	if len(rows) != 3 {
		t.Fatalf("filas del punto A = %d, se esperaban 3 recalculadas", len(rows))
	}
	for _, r := range rows {
		if r.ThresholdVersionID != v.ID || r.ComputedLevel != entities.LevelNaranja || r.RawLevel != entities.LevelNaranja {
			t.Fatalf("fila recalculada con la v2 = %+v; se esperaba NARANJA (cortes de la v2)", r)
		}
	}

	if len(f.evaluations.Rows(pointB)) != rowsB || len(versionsOf(f.fixture, pointB)) != 1 {
		t.Fatal("el punto B no se toca")
	}
	if len(f.publisher.events) != 0 {
		t.Fatal("el backfill NUNCA publica")
	}
}

func TestBackfillReset_WithoutMeasurementsUsesNow(t *testing.T) {
	f := newResetFixture()
	f.resetter.FirstMeasurement = nil

	result, err := f.backfill("development").Backfill(context.Background(), resetCmd())
	if err != nil {
		t.Fatal(err)
	}
	got := versionsOf(f.fixture, pointA)
	if len(got) != 1 || !got[0].ValidFrom.Equal(now) || result.ResetValidFrom == nil || !result.ResetValidFrom.Equal(now) {
		t.Fatalf("valid_from = %+v, informe = %+v; se esperaba now", got, result)
	}
}

// CA-11: sin -point, sin -confirm igual al punto, sin -rebuild, con rango o en
// production se rechaza y NO se escribe nada.
func TestBackfillReset_Refusals(t *testing.T) {
	tests := []struct {
		name string
		env  string
		cmd  func() services.BackfillCommand
		want error
	}{
		{"sin point", "development", func() services.BackfillCommand {
			c := resetCmd()
			c.PointID = 0
			return c
		}, domain.ErrResetRequiresPoint},
		{"sin confirm", "development", func() services.BackfillCommand {
			c := resetCmd()
			c.ConfirmPointID = 0
			return c
		}, domain.ErrResetNotConfirmed},
		{"confirm de otro punto", "development", func() services.BackfillCommand {
			c := resetCmd()
			c.ConfirmPointID = pointB
			return c
		}, domain.ErrResetNotConfirmed},
		{"sin rebuild", "development", func() services.BackfillCommand {
			c := resetCmd()
			c.Rebuild = false
			return c
		}, domain.ErrResetRequiresRebuild},
		{"con rango", "development", func() services.BackfillCommand {
			c := resetCmd()
			c.From = hour(0)
			return c
		}, domain.ErrResetFullHistory},
		{"production", "production", resetCmd, domain.ErrRebuildForbidden},
		{"production con espacios y mayúsculas", " Production ", resetCmd, domain.ErrRebuildForbidden},
	}
	for _, tt := range tests {
		f := newResetFixture()
		feedRojoUnderV1(f)
		if _, err := newBackfill(f.fixture, f.source, "development", 100).Backfill(context.Background(), services.BackfillCommand{PointID: pointA}); err != nil {
			t.Fatal(err)
		}
		versionsBefore, rowsBefore := len(f.thresholds.All()), len(f.evaluations.Rows(pointA))

		_, err := f.backfill(tt.env).Backfill(context.Background(), tt.cmd())
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, se esperaba %v", tt.name, err, tt.want)
		}
		if len(f.thresholds.All()) != versionsBefore || len(f.evaluations.Rows(pointA)) != rowsBefore {
			t.Errorf("%s: no debía escribirse nada", tt.name)
		}
	}
}

func TestBackfillReset_WithoutResetterIsRefused(t *testing.T) {
	f := newResetFixture()
	_, err := newBackfill(f.fixture, f.source, "development", 100).Backfill(context.Background(), resetCmd())
	if !errors.Is(err, domain.ErrResetUnavailable) {
		t.Fatalf("err = %v, se esperaba ErrResetUnavailable", err)
	}
	if len(versionsOf(f.fixture, pointA)) != 2 {
		t.Fatal("no debía escribirse nada")
	}
}

// Una versión programada a futuro no se descarta en silencio.
func TestBackfillReset_RefusesWhenThereIsAFutureVersion(t *testing.T) {
	future := now.Add(24 * time.Hour)
	v2From := hour(1)
	f := newResetFixture()
	f.thresholds = memory.NewThresholdRepository(
		seedVersion(pointA, 1, day1, &v2From, referenceParams()),
		seedVersion(pointA, 2, v2From, &future, v2Params()),
		seedVersion(pointA, 3, future, nil, referenceParams()),
	)
	f.resetter = memory.NewHistoryResetter(f.thresholds, f.evaluations)

	_, err := f.backfill("development").Backfill(context.Background(), resetCmd())
	if !errors.Is(err, domain.ErrResetFutureVersions) || len(f.thresholds.All()) != 3 {
		t.Fatalf("err = %v, versiones = %d; se esperaba rechazo sin escribir", err, len(f.thresholds.All()))
	}
}

func TestBackfillReset_PointWithoutVersionsIsNotFound(t *testing.T) {
	f := newResetFixture()
	cmd := resetCmd()
	cmd.PointID, cmd.ConfirmPointID = 99999, 99999

	if _, err := f.backfill("development").Backfill(context.Background(), cmd); !errors.Is(err, domain.ErrThresholdsNotFound) {
		t.Fatalf("err = %v, se esperaba ErrThresholdsNotFound", err)
	}
}

// Todo en una transacción: si el insert falla, ni las versiones ni las filas se pierden.
func TestBackfillReset_RollsBackWhenInsertFails(t *testing.T) {
	f := newResetFixture()
	feedRojoUnderV1(f)
	if _, err := newBackfill(f.fixture, f.source, "development", 100).Backfill(context.Background(), services.BackfillCommand{PointID: pointA}); err != nil {
		t.Fatal(err)
	}
	rowsBefore := len(f.evaluations.Rows(pointA))
	f.resetter.FailInsert = errors.New("caída de la BD")

	_, err := f.backfill("development").Backfill(context.Background(), resetCmd())
	if !errors.Is(err, domain.ErrPersistingThresholds) {
		t.Fatalf("err = %v, se esperaba ErrPersistingThresholds", err)
	}
	if len(versionsOf(f.fixture, pointA)) != 2 || len(f.evaluations.Rows(pointA)) != rowsBefore {
		t.Fatal("el rollback debe dejar versiones y filas intactas")
	}
}
