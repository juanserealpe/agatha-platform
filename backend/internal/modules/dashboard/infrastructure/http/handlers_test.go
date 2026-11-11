package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/dtos"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

var errDB = errors.New("db caída")

type fakeDevices struct {
	summaries []entities.DeviceSummary
	listErr   error
	exists    bool
}

func (f *fakeDevices) GetAllDeviceSummaries(context.Context) ([]entities.DeviceSummary, error) {
	return f.summaries, f.listErr
}

func (f *fakeDevices) ExistsDevice(context.Context, int64) (bool, error) { return f.exists, nil }

type fakeMeasurements struct{ rows []entities.RawMeasurement }

func (f *fakeMeasurements) GetMeasurements(context.Context, int64, time.Time, time.Time) ([]entities.RawMeasurement, error) {
	return f.rows, nil
}

type fakeSeriesReader struct{ asked []string }

func (f *fakeSeriesReader) Read(_ context.Context, _ int64, defs []entities.SeriesDefinition, _, _ time.Time,
) (map[string][]entities.SeriesPoint, error) {
	for _, d := range defs {
		f.asked = append(f.asked, d.Key)
	}
	return map[string][]entities.SeriesPoint{}, nil
}

func testLogger() *logger.Logger { return logger.New("DashboardHTTPTest", "Error") }

func dashboardService(devices *fakeDevices, measurements *fakeMeasurements) *services.DashboardService {
	log := testLogger()
	return services.NewDashboardService(devices, measurements, services.NewDashboardValidator(log), log)
}

func seriesService(devices *fakeDevices, reader *fakeSeriesReader) *services.SeriesService {
	log := testLogger()
	readers := map[entities.SeriesSource]repositories.SeriesReader{
		entities.SourceMeasurement: reader,
		entities.SourceEvaluation:  reader,
	}
	return services.NewSeriesService(devices, readers, services.NewDashboardValidator(log), log)
}

func serve(h http.HandlerFunc, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("cuerpo de error no es JSON: %q", rec.Body.String())
	}
	return body.Code
}

func validQuery() url.Values {
	end := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	q := url.Values{}
	q.Set("deviceId", "11556")
	q.Set("startDate", end.Add(-24*time.Hour).Format(time.RFC3339))
	q.Set("endDate", end.Format(time.RFC3339))
	return q
}

func TestDeviceHandler_ReturnsTheDevicesAsJSON(t *testing.T) {
	h := NewDeviceHandler(dashboardService(&fakeDevices{summaries: []entities.DeviceSummary{{ID: 1, Name: "PK 1"}}}, nil), testLogger())

	rec := serve(h.GetAllDevices, "/api/devices")

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got []dtos.DeviceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 1 || got[0].Name != "PK 1" {
		t.Fatalf("cuerpo inesperado %q (err %v)", rec.Body.String(), err)
	}
}

func TestDeviceHandler_RepositoryErrorIs500(t *testing.T) {
	h := NewDeviceHandler(dashboardService(&fakeDevices{listErr: errDB}, nil), testLogger())

	rec := serve(h.GetAllDevices, "/api/devices")

	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "CONSULTING_DEVICES" {
		t.Fatalf("status=%d cuerpo=%q", rec.Code, rec.Body.String())
	}
}

// queryCases son las validaciones de parámetros que comparten /measurements y /series.
func queryCases() []struct {
	name     string
	mutate   func(url.Values)
	status   int
	code     string
	contains string
} {
	return []struct {
		name     string
		mutate   func(url.Values)
		status   int
		code     string
		contains string
	}{
		{name: "sin deviceId", mutate: func(q url.Values) { q.Del("deviceId") }, status: 400, code: "INVALID_INPUT"},
		{name: "deviceId no numérico", mutate: func(q url.Values) { q.Set("deviceId", "abc") }, status: 400, code: "INVALID_INPUT"},
		{name: "deviceId cero", mutate: func(q url.Values) { q.Set("deviceId", "0") }, status: 400, code: "INVALID_INPUT"},
		{name: "deviceId negativo", mutate: func(q url.Values) { q.Set("deviceId", "-3") }, status: 400, code: "INVALID_INPUT"},
		{name: "sin startDate", mutate: func(q url.Values) { q.Del("startDate") }, status: 400, code: "INVALID_INPUT"},
		{name: "startDate sin zona horaria", mutate: func(q url.Values) { q.Set("startDate", "2026-10-01T00:00:00") }, status: 400, code: "INVALID_INPUT"},
		{name: "sin endDate", mutate: func(q url.Values) { q.Del("endDate") }, status: 400, code: "INVALID_INPUT"},
		{name: "endDate inválida", mutate: func(q url.Values) { q.Set("endDate", "ayer") }, status: 400, code: "INVALID_INPUT"},
		{
			name: "rango invertido",
			mutate: func(q url.Values) {
				s, e := q.Get("startDate"), q.Get("endDate")
				q.Set("startDate", e)
				q.Set("endDate", s)
			},
			status: 400, code: "INVALID_INPUT",
		},
		{
			name:   "fin futuro",
			mutate: func(q url.Values) { q.Set("endDate", time.Now().Add(48*time.Hour).UTC().Format(time.RFC3339)) },
			status: 400, code: "FUTURE_DATE",
		},
	}
}

func TestMeasurementHandler_QueryValidation(t *testing.T) {
	h := NewMeasurementHandler(dashboardService(&fakeDevices{exists: true}, &fakeMeasurements{}), testLogger())

	for _, tt := range queryCases() {
		t.Run(tt.name, func(t *testing.T) {
			q := validQuery()
			tt.mutate(q)

			rec := serve(h.GetMeasurements, "/api/measurements?"+q.Encode())

			if rec.Code != tt.status || errorCode(t, rec) != tt.code {
				t.Fatalf("status=%d cuerpo=%q, se esperaba %d %s", rec.Code, rec.Body.String(), tt.status, tt.code)
			}
		})
	}
}

func TestMeasurementHandler_OK(t *testing.T) {
	q := validQuery()
	start, _ := time.Parse(time.RFC3339, q.Get("startDate"))
	measurements := &fakeMeasurements{rows: []entities.RawMeasurement{{MetricID: 13, MetricName: "B2", Unit: "-", Timestamp: start, Value: 55}}}
	h := NewMeasurementHandler(dashboardService(&fakeDevices{exists: true}, measurements), testLogger())

	rec := serve(h.GetMeasurements, "/api/measurements?"+q.Encode())

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d cuerpo=%q", rec.Code, rec.Body.String())
	}
	var got dtos.MeasurementsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got.DeviceID != 11556 || len(got.Measurements) != 1 || got.Measurements[0].Data[0].Value != 55 {
		t.Fatalf("respuesta inesperada: %+v", got)
	}
}

func TestMeasurementHandler_UnknownDeviceIs404(t *testing.T) {
	h := NewMeasurementHandler(dashboardService(&fakeDevices{exists: false}, &fakeMeasurements{}), testLogger())

	rec := serve(h.GetMeasurements, "/api/measurements?"+validQuery().Encode())

	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "NOT_FOUND" {
		t.Fatalf("status=%d cuerpo=%q", rec.Code, rec.Body.String())
	}
}

func TestSeriesHandler_QueryValidation(t *testing.T) {
	h := NewSeriesHandler(seriesService(&fakeDevices{exists: true}, &fakeSeriesReader{}), testLogger())

	for _, tt := range queryCases() {
		t.Run(tt.name, func(t *testing.T) {
			q := validQuery()
			tt.mutate(q)

			rec := serve(h.GetSeries, "/api/series?"+q.Encode())

			if rec.Code != tt.status || errorCode(t, rec) != tt.code {
				t.Fatalf("status=%d cuerpo=%q, se esperaba %d %s", rec.Code, rec.Body.String(), tt.status, tt.code)
			}
		})
	}
}

func TestSeriesHandler_UnknownVariableIs400(t *testing.T) {
	h := NewSeriesHandler(seriesService(&fakeDevices{exists: true}, &fakeSeriesReader{}), testLogger())
	q := validQuery()
	q.Set("variables", "b1,presion")

	rec := serve(h.GetSeries, "/api/series?"+q.Encode())

	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "INVALID_INPUT" {
		t.Fatalf("status=%d cuerpo=%q", rec.Code, rec.Body.String())
	}
}

func TestSeriesHandler_VariablesAreTrimmedAndLowercased(t *testing.T) {
	reader := &fakeSeriesReader{}
	h := NewSeriesHandler(seriesService(&fakeDevices{exists: true}, reader), testLogger())
	q := validQuery()
	q.Set("variables", " B2 ,,b1 ")

	rec := serve(h.GetSeries, "/api/series?"+q.Encode())

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d cuerpo=%q", rec.Code, rec.Body.String())
	}
	if len(reader.asked) != 2 || reader.asked[0] != "b1" || reader.asked[1] != "b2" {
		t.Fatalf("series pedidas = %v, se esperaba [b1 b2]", reader.asked)
	}
}

func TestSeriesHandler_WithoutVariablesReturnsTheWholeCatalog(t *testing.T) {
	h := NewSeriesHandler(seriesService(&fakeDevices{exists: true}, &fakeSeriesReader{}), testLogger())

	rec := serve(h.GetSeries, "/api/series?"+validQuery().Encode())

	var got dtos.SeriesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got.Series) != len(entities.SeriesCatalog()) {
		t.Fatalf("status=%d series=%d (err %v)", rec.Code, len(got.Series), err)
	}
}

func TestSeriesHandler_GetVariables(t *testing.T) {
	h := NewSeriesHandler(seriesService(&fakeDevices{}, &fakeSeriesReader{}), testLogger())

	rec := serve(h.GetVariables, "/api/series/variables")

	var got []dtos.VariableResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || len(got) != len(entities.SeriesCatalog()) {
		t.Fatalf("status=%d variables=%d (err %v)", rec.Code, len(got), err)
	}
}

func TestParseVariables(t *testing.T) {
	tests := map[string][]string{
		"":              {},
		" , ,":          {},
		"b1":            {"b1"},
		"B1, Indicador": {"b1", "indicador"},
	}

	for raw, want := range tests {
		got := parseVariables(raw)
		if len(got) != len(want) {
			t.Fatalf("parseVariables(%q) = %v, se esperaba %v", raw, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("parseVariables(%q) = %v, se esperaba %v", raw, got, want)
			}
		}
	}
}
