package services

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	appErrors "github.com/juanserealpe/agatha-platform/backend/internal/shared/errors"
)

func newDashboardService(devices *fakeDevices, measurements *fakeMeasurements) *DashboardService {
	log := testLogger()
	return NewDashboardService(devices, measurements, NewDashboardValidator(log), log)
}

func TestGetAllDevices_MapsEverySummary(t *testing.T) {
	seen := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	battery := 87.5
	devices := &fakeDevices{summaries: []entities.DeviceSummary{
		{ID: 11556, Name: "PK 10+200", LastConnection: &seen, LastBatteryLevel: &battery},
		{ID: 11557, Name: "Sin datos"},
	}}

	got, err := newDashboardService(devices, &fakeMeasurements{}).GetAllDevices(context.Background())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("se esperaban 2 dispositivos, hay %d", len(got))
	}
	if got[0].ID != 11556 || got[0].Name != "PK 10+200" || !got[0].LastConnection.Equal(seen) || *got[0].LastBatteryLevel != battery {
		t.Fatalf("primer dispositivo mal mapeado: %+v", got[0])
	}
	if got[1].LastConnection != nil || got[1].LastBatteryLevel != nil {
		t.Fatalf("los campos ausentes deben quedar nil: %+v", got[1])
	}
}

func TestGetAllDevices_EmptyIsAnEmptyListNotNil(t *testing.T) {
	got, err := newDashboardService(&fakeDevices{}, &fakeMeasurements{}).GetAllDevices(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("se esperaba lista vacía no nil, se obtuvo %v (err %v)", got, err)
	}
}

func TestGetAllDevices_RepositoryErrorIsHidden(t *testing.T) {
	_, err := newDashboardService(&fakeDevices{listErr: errDB}, &fakeMeasurements{}).GetAllDevices(context.Background())
	if !errors.Is(err, domain.ErrConsultingDevices) {
		t.Fatalf("error = %v, se esperaba ErrConsultingDevices", err)
	}
}

func TestGetMeasurements_GroupsByMetricKeepingOrder(t *testing.T) {
	start, end := pastRange()
	t1, t2 := start.Add(time.Hour), start.Add(2*time.Hour)
	measurements := &fakeMeasurements{rows: []entities.RawMeasurement{
		{MetricID: 13, MetricName: "B2", Unit: "-", Timestamp: t1, Value: 50},
		{MetricID: 20, MetricName: "Temperatura", Unit: "°C", Timestamp: t1, Value: 21.5},
		{MetricID: 13, MetricName: "B2", Unit: "-", Timestamp: t2, Value: 60},
	}}

	got, err := newDashboardService(&fakeDevices{exists: true}, measurements).
		GetMeasurements(context.Background(), 11556, start, end)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if got.DeviceID != 11556 || !got.StartDate.Equal(start) || !got.EndDate.Equal(end) {
		t.Fatalf("cabecera mal armada: %+v", got)
	}
	if len(got.Measurements) != 2 {
		t.Fatalf("se esperaban 2 métricas, hay %d", len(got.Measurements))
	}

	// El servicio agrupa en un map: el orden entre métricas no está garantizado.
	sort.Slice(got.Measurements, func(a, b int) bool { return got.Measurements[a].MetricID < got.Measurements[b].MetricID })

	b2 := got.Measurements[0]
	if b2.MetricID != 13 || b2.MetricName != "B2" || len(b2.Data) != 2 || b2.Data[0].Value != 50 || b2.Data[1].Value != 60 {
		t.Fatalf("B2 mal agrupada: %+v", b2)
	}
	if temp := got.Measurements[1]; temp.Unit != "°C" || len(temp.Data) != 1 || !temp.Data[0].Timestamp.Equal(t1) {
		t.Fatalf("temperatura mal agrupada: %+v", temp)
	}
}

func TestGetMeasurements_NoRowsIsAnEmptyList(t *testing.T) {
	start, end := pastRange()
	got, err := newDashboardService(&fakeDevices{exists: true}, &fakeMeasurements{}).
		GetMeasurements(context.Background(), 1, start, end)
	if err != nil || got.Measurements == nil || len(got.Measurements) != 0 {
		t.Fatalf("se esperaba lista vacía no nil, se obtuvo %+v (err %v)", got, err)
	}
}

func TestGetMeasurements_Rejections(t *testing.T) {
	start, end := pastRange()
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name         string
		start, end   time.Time
		devices      *fakeDevices
		measurements *fakeMeasurements
		want         error
		readsDB      bool
	}{
		{name: "rango invertido", start: end, end: start, devices: &fakeDevices{exists: true}, want: domain.ErrInvalidDateRange},
		{name: "inicio futuro", start: future, end: future.Add(time.Hour), devices: &fakeDevices{exists: true}, want: domain.ErrStartDateFuture},
		{name: "fin futuro", start: start, end: future, devices: &fakeDevices{exists: true}, want: domain.ErrEndDateFuture},
		{name: "dispositivo inexistente", start: start, end: end, devices: &fakeDevices{exists: false}, want: appErrors.ErrDeviceNotFound},
		{name: "error verificando el dispositivo", start: start, end: end, devices: &fakeDevices{existsErr: errDB}, want: domain.ErrConsultingMeasurements},
		{
			name: "error consultando mediciones", start: start, end: end,
			devices: &fakeDevices{exists: true}, measurements: &fakeMeasurements{err: errDB},
			want: domain.ErrConsultingMeasurements, readsDB: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			measurements := tt.measurements
			if measurements == nil {
				measurements = &fakeMeasurements{}
			}

			got, err := newDashboardService(tt.devices, measurements).GetMeasurements(context.Background(), 7, tt.start, tt.end)
			if got != nil || !errors.Is(err, tt.want) {
				t.Fatalf("respuesta=%v error=%v, se esperaba error %v", got, err, tt.want)
			}
			if tt.readsDB != (measurements.calls > 0) {
				t.Fatalf("consultas de mediciones = %d, se esperaba consultar: %v", measurements.calls, tt.readsDB)
			}
		})
	}
}
