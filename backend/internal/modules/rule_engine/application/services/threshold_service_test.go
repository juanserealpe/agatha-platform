package services_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/memory"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

const (
	pointA int64 = 11556
	pointB int64 = 11557
	leak         = entities.EventTypeLeak
)

var (
	now  = time.Date(2026, 10, 19, 15, 10, 0, 0, time.UTC)
	day1 = time.Date(2026, 7, 8, 5, 15, 0, 0, time.UTC)
)

func fixedClock() ports.Clock { return ports.ClockFunc(func() time.Time { return now }) }

func testLogger() *logger.Logger { return logger.New("RuleEngineTest", "Error") }

func seedVersion(pointID int64, version int, from time.Time, to *time.Time, params entities.ThresholdParams) entities.ThresholdVersion {
	return entities.ThresholdVersion{
		MeasuringPointID: pointID,
		EventType:        leak,
		Version:          version,
		ThresholdParams:  params,
		ValidFrom:        from,
		ValidTo:          to,
		Reason:           entities.SeedReason,
		CreatedBy:        entities.SystemCreatedBy,
	}
}

func newThresholdService(repo *memory.ThresholdRepository) *services.ThresholdService {
	return services.NewThresholdService(repo, fixedClock(), testLogger())
}

func TestCreateThresholdVersion_ClosesCurrentAndCreatesNextInOneTransaction(t *testing.T) {
	custom := entities.DefaultThresholdParams()
	custom.Amarillo = 0.20 // distinto del default: se copia de la vigente, no de la plantilla
	repo := memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, custom))

	created, err := newThresholdService(repo).CreateThresholdVersion(context.Background(), services.CreateThresholdVersionCommand{
		PointID:   pointA,
		Patch:     entities.ThresholdPatch{Rojo: entities.Some(0.40)},
		Reason:    "ajuste tras revisión con el analista",
		CreatedBy: "analista@og",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if created.Version != 2 || !created.ValidFrom.Equal(now) || created.ValidTo != nil || created.ID == 0 {
		t.Fatalf("versión creada = %+v", created)
	}
	if created.Rojo != 0.40 || created.Amarillo != 0.20 || created.Naranja != custom.Naranja || created.MinHours != custom.MinHours {
		t.Fatalf("parámetros = %+v: el informado se aplica y los omitidos se copian de la vigente", created.ThresholdParams)
	}
	if created.Reason != "ajuste tras revisión con el analista" || created.CreatedBy != "analista@og" || created.EventType != leak {
		t.Fatalf("trazabilidad = %+v", created)
	}

	all := repo.All()
	if len(all) != 2 {
		t.Fatalf("versiones = %d, se esperaban 2", len(all))
	}
	if all[0].ValidTo == nil || !all[0].ValidTo.Equal(now) {
		t.Fatalf("la v1 debía cerrarse en now: valid_to = %v", all[0].ValidTo)
	}
	if all[0].Rojo != custom.Rojo {
		t.Fatal("la v1 no debe modificarse salvo valid_to")
	}
}

func TestCreateThresholdVersion_ValidationErrorsWriteNothing(t *testing.T) {
	tests := []struct {
		name string
		cmd  services.CreateThresholdVersionCommand
		want error
	}{
		{"fecha en el pasado", services.CreateThresholdVersionCommand{
			PointID: pointA, ValidFrom: now.Add(-time.Minute), Reason: "r", CreatedBy: "c",
		}, domain.ErrValidFromInPast},
		{"orden inválido", services.CreateThresholdVersionCommand{
			PointID: pointA, Patch: entities.ThresholdPatch{Naranja: entities.Some(0.10)}, Reason: "r", CreatedBy: "c",
		}, domain.ErrThresholdOrder},
		{"margen fuera de rango", services.CreateThresholdVersionCommand{
			PointID: pointA, Patch: entities.ThresholdPatch{ExitMargin: entities.Some(0.5)}, Reason: "r", CreatedBy: "c",
		}, domain.ErrExitMarginRange},
		{"horas inválidas", services.CreateThresholdVersionCommand{
			PointID: pointA, Patch: entities.ThresholdPatch{MinHours: entities.Some(0)}, Reason: "r", CreatedBy: "c",
		}, domain.ErrHoursRange},
		{"sin motivo", services.CreateThresholdVersionCommand{PointID: pointA, CreatedBy: "c"}, domain.ErrMissingAuthorship},
		{"punto inválido", services.CreateThresholdVersionCommand{PointID: 0, Reason: "r", CreatedBy: "c"}, domain.ErrInvalidMeasuringPointID},
		{"evento no soportado", services.CreateThresholdVersionCommand{
			PointID: pointA, EventType: "OTRO", Reason: "r", CreatedBy: "c",
		}, domain.ErrInvalidEventType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams()))

			_, err := newThresholdService(repo).CreateThresholdVersion(context.Background(), tt.cmd)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, se esperaba %v", err, tt.want)
			}

			all := repo.All()
			if len(all) != 1 || all[0].ValidTo != nil {
				t.Fatalf("no debía escribirse nada (ni cerrar la vigente): %+v", all)
			}
		})
	}
}

func TestCreateThresholdVersion_FirstVersionStartsFromTemplate(t *testing.T) {
	repo := memory.NewThresholdRepository()

	created, err := newThresholdService(repo).CreateThresholdVersion(context.Background(), services.CreateThresholdVersionCommand{
		PointID: pointA, Patch: entities.ThresholdPatch{Amarillo: entities.Some(0.19)}, Reason: "r", CreatedBy: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.Amarillo != 0.19 || created.Naranja != 0.25 {
		t.Fatalf("primera versión = %+v", created)
	}
}

func TestCreateThresholdVersion_FutureValidFromKeepsCurrentUntilThen(t *testing.T) {
	repo := memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams()))
	svc := newThresholdService(repo)
	ctx := context.Background()
	tomorrow := now.Add(24 * time.Hour)

	if _, err := svc.CreateThresholdVersion(ctx, services.CreateThresholdVersionCommand{
		PointID: pointA, ValidFrom: tomorrow, Patch: entities.ThresholdPatch{Rojo: entities.Some(0.5)}, Reason: "r", CreatedBy: "c",
	}); err != nil {
		t.Fatal(err)
	}

	current, err := svc.GetCurrentThresholds(ctx, pointA, leak)
	if err != nil || current.Version != 1 {
		t.Fatalf("la vigente hoy sigue siendo la v1: %+v, %v", current, err)
	}

	// Otra versión debe empezar después de la programada.
	_, err = svc.CreateThresholdVersion(ctx, services.CreateThresholdVersionCommand{
		PointID: pointA, Reason: "r", CreatedBy: "c",
	})
	if !errors.Is(err, domain.ErrValidFromNotAfterCurrent) {
		t.Fatalf("err = %v, se esperaba ErrValidFromNotAfterCurrent", err)
	}
}

// Caso 4: el segundo POST concurrente (lock NOWAIT ocupado, UNIQUE o EXCLUDE) es 409.
func TestCreateThresholdVersion_ConcurrentWriteIsConflict(t *testing.T) {
	repo := memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams()))
	repo.FailNextTransaction = fmt.Errorf("lock ocupado: %w", repositories.ErrConflict)

	_, err := newThresholdService(repo).CreateThresholdVersion(context.Background(), services.CreateThresholdVersionCommand{
		PointID: pointA, Patch: entities.ThresholdPatch{Amarillo: entities.Some(0.16)}, Reason: "r", CreatedBy: "c",
	})
	if !errors.Is(err, domain.ErrThresholdConflict) {
		t.Fatalf("err = %v, se esperaba ErrThresholdConflict", err)
	}
	if got := repo.All(); len(got) != 1 || got[0].ValidTo != nil {
		t.Fatal("un conflicto no escribe nada")
	}
}

// Caso 1 + columnas v2: el parche de amarillo copia los campos nuevos de la vigente.
func TestCreateThresholdVersion_AmarilloV2KeepsNewColumns(t *testing.T) {
	base := entities.DefaultThresholdParams()
	base.PersistenceMode = entities.PersistenceWindow
	base.PersistenceWindowHours, base.PersistenceMinHours = intPtr(24), intPtr(18)
	repo := memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, base))

	v, err := newThresholdService(repo).CreateThresholdVersion(context.Background(), services.CreateThresholdVersionCommand{
		PointID: pointA, Patch: entities.ThresholdPatch{Amarillo: entities.Some(0.16)}, Reason: "amarillo a 0,16", CreatedBy: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Amarillo != 0.16 || v.ExitCutoff() != 0.14 || !v.IsWindow() || *v.PersistenceMinHours != 18 {
		t.Fatalf("versión = %+v", v.ThresholdParams)
	}
}

func TestCreateThresholdVersion_StorageFailureIsPersistingError(t *testing.T) {
	repo := memory.NewThresholdRepository(seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams()))
	repo.FailNextTransaction = errors.New("db caída")

	_, err := newThresholdService(repo).CreateThresholdVersion(context.Background(), services.CreateThresholdVersionCommand{
		PointID: pointA, Reason: "r", CreatedBy: "c",
	})
	if !errors.Is(err, domain.ErrPersistingThresholds) {
		t.Fatalf("err = %v, se esperaba ErrPersistingThresholds", err)
	}
}

func TestCreateThresholdVersionBulk_IsAtomic(t *testing.T) {
	strictB := entities.DefaultThresholdParams()
	strictB.Amarillo, strictB.Naranja, strictB.Rojo = 0.23, 0.30, 0.40

	repo := memory.NewThresholdRepository(
		seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams()),
		seedVersion(pointB, 1, day1, nil, strictB),
	)

	// Naranja = 0.22 es válido para A (amarillo 0.18) pero no para B (amarillo 0.23).
	_, err := newThresholdService(repo).CreateThresholdVersionBulk(context.Background(), services.CreateThresholdVersionBulkCommand{
		PointIDs: []int64{pointA, pointB},
		Patch:    entities.ThresholdPatch{Naranja: entities.Some(0.22)},
		Reason:   "r", CreatedBy: "c",
	})
	if !errors.Is(err, domain.ErrThresholdOrder) {
		t.Fatalf("err = %v, se esperaba ErrThresholdOrder", err)
	}

	for _, v := range repo.All() {
		if v.Version != 1 || v.ValidTo != nil {
			t.Fatalf("el bulk debía revertirse por completo: %+v", v)
		}
	}
}

func TestCreateThresholdVersionBulk_WithoutPointsAppliesToAllPointsWithVersions(t *testing.T) {
	repo := memory.NewThresholdRepository(
		seedVersion(pointA, 1, day1, nil, entities.DefaultThresholdParams()),
		seedVersion(pointB, 1, day1, nil, entities.DefaultThresholdParams()),
	)

	created, err := newThresholdService(repo).CreateThresholdVersionBulk(context.Background(), services.CreateThresholdVersionBulkCommand{
		Patch:  entities.ThresholdPatch{ExitHours: entities.Some(3)},
		Reason: "r", CreatedBy: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || created[0].MeasuringPointID != pointA || created[1].MeasuringPointID != pointB {
		t.Fatalf("creadas = %+v", created)
	}
	for _, v := range created {
		if v.Version != 2 || v.ExitHours != 3 {
			t.Fatalf("versión = %+v", v)
		}
	}
}

func TestGetThresholdTimeline_ClipsEndsWithSeveralVersions(t *testing.T) {
	d10 := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	d20 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	p2 := entities.DefaultThresholdParams()
	p2.Rojo = 0.40

	repo := memory.NewThresholdRepository(
		seedVersion(pointA, 1, day1, &d10, entities.DefaultThresholdParams()),
		seedVersion(pointA, 2, d10, &d20, p2),
		seedVersion(pointA, 3, d20, nil, entities.DefaultThresholdParams()),
	)
	svc := newThresholdService(repo)
	ctx := context.Background()

	from, to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	segments, err := svc.GetThresholdTimeline(ctx, pointA, leak, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 3 || !segments[0].From.Equal(from) || !segments[2].To.Equal(to) ||
		!segments[1].From.Equal(d10) || !segments[1].To.Equal(d20) || segments[1].Version.Rojo != 0.40 {
		t.Fatalf("tramos = %+v", segments)
	}

	if _, err := svc.GetThresholdTimeline(ctx, pointA, leak, to, from); !errors.Is(err, domain.ErrInvalidTimeRange) {
		t.Fatalf("rango invertido: err = %v", err)
	}

	history, err := svc.GetThresholdHistory(ctx, pointA, leak)
	if err != nil || len(history) != 3 || history[0].Version != 1 || history[2].Version != 3 {
		t.Fatalf("historial = %+v, %v", history, err)
	}
}

func TestGetCurrentThresholds_NotFound(t *testing.T) {
	_, err := newThresholdService(memory.NewThresholdRepository()).GetCurrentThresholds(context.Background(), pointA, leak)
	if !errors.Is(err, domain.ErrThresholdsNotFound) {
		t.Fatalf("err = %v, se esperaba ErrThresholdsNotFound", err)
	}
}

func TestEnsureInitialVersion_CreatesV1FromNowOnlyOnce(t *testing.T) {
	repo := memory.NewThresholdRepository()
	svc := newThresholdService(repo)
	ctx := context.Background()

	created, err := svc.EnsureInitialVersion(ctx, pointA, leak)
	if err != nil || !created {
		t.Fatalf("created = %v, err = %v", created, err)
	}
	created, err = svc.EnsureInitialVersion(ctx, pointA, leak)
	if err != nil || created {
		t.Fatalf("segunda llamada: created = %v, err = %v (no debe crear otra)", created, err)
	}

	all := repo.All()
	if len(all) != 1 || all[0].Version != 1 || !all[0].ValidFrom.Equal(now) ||
		all[0].CreatedBy != entities.SystemCreatedBy || all[0].Reason != entities.FirstUseSeedReason ||
		all[0].ThresholdParams.Amarillo != entities.DefaultThresholdParams().Amarillo {
		t.Fatalf("versión inicial = %+v", all)
	}
}

// referenceParams son los parámetros del escenario de referencia de la
// especificación (amarillo 0,18, salida 0,16): los escenarios de este archivo
// están escritos sobre esos cortes. La plantilla de producción
// (entities.DefaultThresholdParams) ya usa amarillo 0,16.
func referenceParams() entities.ThresholdParams {
	p := entities.DefaultThresholdParams()
	p.Amarillo = 0.18
	return p
}
