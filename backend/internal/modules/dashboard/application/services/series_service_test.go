package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

func newSeriesService(devices *fakeDevices, readers map[entities.SeriesSource]repositories.SeriesReader) *SeriesService {
	log := testLogger()
	return NewSeriesService(devices, readers, NewDashboardValidator(log), log)
}

func TestGetVariables_ListsTheWholeCatalog(t *testing.T) {
	got := newSeriesService(&fakeDevices{}, nil).GetVariables()
	catalog := entities.SeriesCatalog()

	if len(got) != len(catalog) {
		t.Fatalf("variables = %d, catálogo = %d", len(got), len(catalog))
	}
	for i, v := range got {
		c := catalog[i]
		if v.Key != c.Key || v.Name != c.Name || v.Unit != c.Unit || v.Source != string(c.Source) {
			t.Fatalf("variable %d = %+v, se esperaba %+v", i, v, c)
		}
	}
}

func TestGetSeries_ReadsEachSourceOnceAndKeepsCatalogOrder(t *testing.T) {
	start, end := pastRange()
	ts := start.Add(time.Hour)

	measurement := &fakeSeriesReader{points: map[string][]entities.SeriesPoint{
		"b1": {{Timestamp: ts, Value: 10}},
		"b2": {{Timestamp: ts, Value: 20}, {Timestamp: ts.Add(time.Hour), Value: 21}},
	}}
	evaluation := &fakeSeriesReader{points: map[string][]entities.SeriesPoint{
		"indicador": {{Timestamp: ts, Value: 0.42}},
	}}
	svc := newSeriesService(&fakeDevices{exists: true}, map[entities.SeriesSource]repositories.SeriesReader{
		entities.SourceMeasurement: measurement,
		entities.SourceEvaluation:  evaluation,
	})

	got, err := svc.GetSeries(context.Background(), 11556, start, end, []string{"indicador", "b2", "b1"})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(measurement.asked) != 1 || len(evaluation.asked) != 1 {
		t.Fatalf("cada fuente debe leerse una sola vez: medición=%v evaluación=%v", measurement.asked, evaluation.asked)
	}
	if want := []string{"b1", "b2"}; len(measurement.asked[0]) != 2 || measurement.asked[0][0] != want[0] || measurement.asked[0][1] != want[1] {
		t.Fatalf("al lector de mediciones se le pidió %v, se esperaba %v", measurement.asked[0], want)
	}

	keys := []string{}
	for _, s := range got.Series {
		keys = append(keys, s.Key)
	}
	if len(keys) != 3 || keys[0] != "b1" || keys[1] != "b2" || keys[2] != "indicador" {
		t.Fatalf("series = %v, se esperaba el orden del catálogo [b1 b2 indicador]", keys)
	}
	if got.Series[1].Source != "measurement" || len(got.Series[1].Data) != 2 || got.Series[1].Data[1].Value != 21 {
		t.Fatalf("b2 mal armada: %+v", got.Series[1])
	}
	if got.Series[2].Source != "evaluation" || got.Series[2].Data[0].Value != 0.42 {
		t.Fatalf("indicador mal armado: %+v", got.Series[2])
	}
}

func TestGetSeries_SeriesWithoutPointsHasEmptyData(t *testing.T) {
	start, end := pastRange()
	svc := newSeriesService(&fakeDevices{exists: true}, map[entities.SeriesSource]repositories.SeriesReader{
		entities.SourceMeasurement: &fakeSeriesReader{points: map[string][]entities.SeriesPoint{}},
	})

	got, err := svc.GetSeries(context.Background(), 1, start, end, []string{"b1"})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Series[0].Data == nil || len(got.Series[0].Data) != 0 {
		t.Fatalf("una serie sin puntos debe tener data vacía no nil: %+v", got.Series[0])
	}
}

func TestGetSeries_Rejections(t *testing.T) {
	start, end := pastRange()
	future := time.Now().Add(time.Hour)
	okReaders := func() map[entities.SeriesSource]repositories.SeriesReader {
		return map[entities.SeriesSource]repositories.SeriesReader{
			entities.SourceMeasurement: &fakeSeriesReader{},
			entities.SourceEvaluation:  &fakeSeriesReader{},
		}
	}

	tests := []struct {
		name       string
		start, end time.Time
		keys       []string
		devices    *fakeDevices
		readers    map[entities.SeriesSource]repositories.SeriesReader
		want       error
	}{
		{name: "rango invertido", start: end, end: start, devices: &fakeDevices{exists: true}, readers: okReaders(), want: domain.ErrInvalidDateRange},
		{name: "inicio futuro", start: future, end: future.Add(time.Hour), devices: &fakeDevices{exists: true}, readers: okReaders(), want: domain.ErrStartDateFuture},
		{name: "fin futuro", start: start, end: future, devices: &fakeDevices{exists: true}, readers: okReaders(), want: domain.ErrEndDateFuture},
		{name: "variable desconocida", start: start, end: end, keys: []string{"presion"}, devices: &fakeDevices{exists: true}, readers: okReaders(), want: domain.ErrUnknownVariable},
		{name: "dispositivo inexistente", start: start, end: end, devices: &fakeDevices{}, readers: okReaders(), want: appErrors.ErrDeviceNotFound},
		{name: "error verificando el dispositivo", start: start, end: end, devices: &fakeDevices{existsErr: errDB}, readers: okReaders(), want: domain.ErrConsultingSeries},
		{
			name: "falta el lector de una fuente", start: start, end: end, keys: []string{"indicador"},
			devices: &fakeDevices{exists: true},
			readers: map[entities.SeriesSource]repositories.SeriesReader{entities.SourceMeasurement: &fakeSeriesReader{}},
			want:    domain.ErrConsultingSeries,
		},
		{
			name: "el lector falla", start: start, end: end, keys: []string{"b1"},
			devices: &fakeDevices{exists: true},
			readers: map[entities.SeriesSource]repositories.SeriesReader{entities.SourceMeasurement: &fakeSeriesReader{err: errDB}},
			want:    domain.ErrConsultingSeries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSeriesService(tt.devices, tt.readers).GetSeries(context.Background(), 5, tt.start, tt.end, tt.keys)
			if got != nil || !errors.Is(err, tt.want) {
				t.Fatalf("respuesta=%v error=%v, se esperaba error %v", got, err, tt.want)
			}
		})
	}
}

func TestGetSeries_UnknownVariableNeverTouchesTheDatabase(t *testing.T) {
	start, end := pastRange()
	devices := &fakeDevices{exists: true}

	_, _ = newSeriesService(devices, nil).GetSeries(context.Background(), 5, start, end, []string{"presion"})

	if len(devices.existsCall) != 0 {
		t.Fatalf("no se debe consultar el dispositivo si una variable es desconocida: %v", devices.existsCall)
	}
}
