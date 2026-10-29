package services_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/memory"
)

func TestParseFilter_ColombianDayBoundariesAndMultiselect(t *testing.T) {
	f, err := services.ParseFilter(services.FilterParams{
		PointID: pointA, From: "01/08/2026", To: "2026-08-31",
		MaxLevels: []string{"naranja,ROJO"}, Statuses: []string{"SIN_CLASIFICAR"}, Events: []string{"FUGA"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 01/08 00:00 Colombia = 05:00 UTC; 31/08 23:59 ⇒ fin exclusivo 01/09 05:00 UTC.
	if !f.From.Equal(time.Date(2026, 8, 1, 5, 0, 0, 0, time.UTC)) || !f.To.Equal(time.Date(2026, 9, 1, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("rango = [%v, %v)", f.From, f.To)
	}
	if len(f.MaxLevels) != 2 || f.MaxLevels[0] != entities.LevelNaranja || len(f.Statuses) != 1 || len(f.Events) != 1 {
		t.Fatalf("filtro = %+v", f)
	}
}

func TestParseFilter_Rejections(t *testing.T) {
	for name, p := range map[string]services.FilterParams{
		"fin antes de inicio": {From: "31/08/2026", To: "01/08/2026"},
		"fecha inválida":      {From: "2026/08/01"},
		"nivel NORMAL":        {MaxLevels: []string{"NORMAL"}},
		"estado desconocido":  {Statuses: []string{"CERRADA"}},
		"evento desconocido":  {Events: []string{"SISMO"}},
	} {
		if _, err := services.ParseFilter(p); !errors.Is(err, domain.ErrInvalidFilter) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// Un solo día es válido (de 00:00 a 23:59).
	if _, err := services.ParseFilter(services.FilterParams{From: "24/08/2026", To: "24/08/2026"}); err != nil {
		t.Fatalf("mismo día: %v", err)
	}
}

type fakeQueries struct {
	ports.AlertQueries
	count   int64
	details []ports.AlertDetail
	active  []ports.AlertSummary
	filter  ports.SearchFilter
}

func (f *fakeQueries) Count(_ context.Context, filter ports.SearchFilter) (int64, error) {
	f.filter = filter
	return f.count, nil
}

func (f *fakeQueries) SearchDetails(context.Context, ports.SearchFilter, int) ([]ports.AlertDetail, error) {
	return f.details, nil
}

func (f *fakeQueries) Active(context.Context) ([]ports.AlertSummary, error) { return f.active, nil }

type captureWriter struct {
	sheets []ports.Sheet
}

func (c *captureWriter) Write(w io.Writer, sheets []ports.Sheet) error {
	c.sheets = sheets
	_, err := w.Write([]byte("xlsx"))
	return err
}

func newQueryService(q *fakeQueries, w *captureWriter) *services.QueryService {
	return services.NewQueryService(q, w, ports.ClockFunc(func() time.Time { return h(100) }), testLogger())
}

// Escenario 19: más de 10.000 alertas ⇒ 400 sin archivo.
func TestExport_TooLargeWritesNothing(t *testing.T) {
	q := &fakeQueries{count: 10_001}
	var buf bytes.Buffer
	if err := newQueryService(q, &captureWriter{}).Export(context.Background(), services.FilterParams{}, &buf); !errors.Is(err, domain.ErrExportTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatal("no debe escribirse nada")
	}
}

// Escenario 19b: el archivo usa los mismos filtros que la búsqueda.
func TestExport_SheetsFollowTheSearch(t *testing.T) {
	ended := h(10)
	classifiedAt := h(5)
	q := &fakeQueries{count: 1, details: []ports.AlertDetail{{
		AlertSummary: ports.AlertSummary{PK: "PK34+500", Alert: entities.Alert{
			ID: 123, Code: "ALR-000123", MeasuringPointID: pointA, EventType: entities.EventTypeLeak,
			State: entities.AlertFinished, StartedAt: h(0), EndedAt: &ended,
			CurrentLevel: entities.LevelAmarillo, MaxLevel: entities.LevelNaranja,
			Management: entities.Management{Status: entities.StatusClassifiedAndLabel, Classification: entities.ClassificationConfirmed,
				ClassifiedAt: &classifiedAt, ClassifiedOrigin: entities.OriginMobile, ClassifiedDevice: "Celular 1",
				Label: entities.LabelOther, LabelOtherText: "Prueba hidrostática"},
		}},
		Segments: []ports.SegmentWithMessages{
			{Segment: entities.Segment{Level: entities.LevelAmarillo, StartedAt: h(0)}},
			{Segment: entities.Segment{Level: entities.LevelNaranja, StartedAt: h(3)}},
		},
		Observations: []entities.Observation{{Text: "Sin hallazgos", Origin: entities.OriginWeb, ReceivedAt: h(6)}},
	}}}
	w := &captureWriter{}
	var buf bytes.Buffer
	p := services.FilterParams{PointID: pointA, From: "24/08/2026", MaxLevels: []string{"NARANJA"}}
	if err := newQueryService(q, w).Export(context.Background(), p, &buf); err != nil {
		t.Fatal(err)
	}
	if q.filter.PointID != pointA || len(q.filter.MaxLevels) != 1 {
		t.Fatalf("filtro = %+v", q.filter)
	}
	if len(w.sheets) != 2 || w.sheets[0].Name != "Alertas" || w.sheets[1].Name != "Parámetros" {
		t.Fatalf("hojas = %+v", w.sheets)
	}
	row := strings.Join(w.sheets[0].Rows[1], " | ")
	for _, want := range []string{"ALR-000123", "PK34+500", "24/08/2026 11:00", "—", "Naranja", "Confirmada",
		"Clasificada con etiqueta", "Prueba hidrostática", "Sin hallazgos", "App móvil (Celular 1)", "Amarillo 24/08/2026 11:00 → Naranja 24/08/2026 14:00"} {
		if !strings.Contains(row, want) {
			t.Errorf("la fila no tiene %q: %s", want, row)
		}
	}
	if total := w.sheets[1].Rows[len(w.sheets[1].Rows)-1]; total[1] != "1" {
		t.Fatalf("total = %v", total)
	}
}

func TestActive_GroupsByDeviceWithHighestLevel(t *testing.T) {
	q := &fakeQueries{active: []ports.AlertSummary{
		{Alert: entities.Alert{ID: 1, MeasuringPointID: pointA, State: entities.AlertActive, CurrentLevel: entities.LevelAmarillo}},
		{Alert: entities.Alert{ID: 2, MeasuringPointID: pointA, State: entities.AlertActive, CurrentLevel: entities.LevelRojo}},
	}}
	out, err := newQueryService(q, &captureWriter{}).Active(context.Background())
	if err != nil || len(out) != 1 || out[0].Level != entities.LevelRojo || !out[0].Blinking || len(out[0].Alerts) != 2 {
		t.Fatalf("activas = %+v, %v", out, err)
	}
}

func TestPolicy_CreateCopiesCurrentAndRejectsPast(t *testing.T) {
	store := memory.NewPolicyStore()
	now := h(0)
	svc := services.NewPolicyService(store, ports.ClockFunc(func() time.Time { return now }), testLogger())
	six := 4
	point := pointA
	created, err := svc.Create(context.Background(), services.CreatePolicyCommand{
		PointID: &point, Patch: entities.PolicyPatch{DowngradeHours: &six}, Reason: "piloto", CreatedBy: "jvarona",
	})
	if err != nil || created.DowngradeHours != 4 || created.MergeWindowHours != 24 || created.Version != 1 {
		t.Fatalf("creada = %+v, %v", created, err)
	}
	current, _ := svc.Current(context.Background(), &point, "")
	if current.ID != created.ID {
		t.Fatal("la del punto reemplaza a la global")
	}
	if _, err := svc.Create(context.Background(), services.CreatePolicyCommand{ValidFrom: now.Add(-time.Hour), Reason: "x", CreatedBy: "y"}); !errors.Is(err, domain.ErrValidFromInPast) {
		t.Fatalf("pasado: err = %v", err)
	}
	rojo := entities.LevelRojo
	if _, err := svc.Create(context.Background(), services.CreatePolicyCommand{Patch: entities.PolicyPatch{InterventionSilentMaxLevel: &rojo}, Reason: "x", CreatedBy: "y"}); !errors.Is(err, domain.ErrInvalidPolicy) {
		t.Fatalf("rojo silencioso: err = %v", err)
	}
}
