package services

import (
	"context"
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// Golden 15.5 (bloqueante para cerrar T8). Requiere el fixture del analista
// testdata/study_11556.csv (timestamp RFC3339, B1, B2, Planitud; vacío = ausente)
// y BASELINE_STUDY_ASOF (RFC3339) con el asOf de su ejecución. Sin ellos se salta.
const studyFixture = "testdata/study_11556.csv"

func loadStudy(t *testing.T) ([]features.Sample, time.Time) {
	t.Helper()

	asOfEnv := os.Getenv("BASELINE_STUDY_ASOF")
	file, err := os.Open(studyFixture)
	if err != nil || asOfEnv == "" {
		t.Skip("falta el fixture del analista (testdata/study_11556.csv) o BASELINE_STUDY_ASOF")
	}
	defer file.Close()

	studyAsOf, err := time.Parse(time.RFC3339, asOfEnv)
	if err != nil {
		t.Fatalf("BASELINE_STUDY_ASOF inválido: %v", err)
	}

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("leyendo fixture: %v", err)
	}

	parse := func(s string) (float64, bool) {
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	}

	samples := make([]features.Sample, 0, len(records))
	for i, r := range records {
		if i == 0 {
			continue // encabezado
		}
		ts, err := time.Parse(time.RFC3339, r[0])
		if err != nil {
			t.Fatalf("fila %d: timestamp %q", i, r[0])
		}
		s := features.Sample{T: ts.UTC()}
		s.B1, s.HasB1 = parse(r[1])
		s.B2, s.HasB2 = parse(r[2])
		s.Pl, s.HasPl = parse(r[3])
		samples = append(samples, s)
	}

	return samples, studyAsOf
}

func TestGoldenStudy11556(t *testing.T) {
	samples, studyAsOf := loadStudy(t)

	h := &fakeHistory{samples: map[int64][]features.Sample{point: samples}}
	res, err := newService(h, &fakeBaselines{}).Recalculate(context.Background(),
		RecalculateCommand{PointID: point, AsOf: studyAsOf, Trigger: entities.TriggerManual, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	sig := res.Version.Signal
	if sig.ThetaD != 67 || sig.MedianB2 != 54 {
		t.Errorf("θ_D = %v, mediana = %v; se esperaba 67 y 54", sig.ThetaD, sig.MedianB2)
	}
	if math.Abs(res.Estimation.ExcludedRatio*100-21.8) > 0.5 {
		t.Errorf("excluido = %.2f %%, se esperaba 21,8 ± 0,5", res.Estimation.ExcludedRatio*100)
	}
	if res.Estimation.Passes != 4 || !res.Estimation.Converged || res.Estimation.CapReached {
		t.Errorf("estimación = %+v; se esperaba convergencia en 4 pasadas sin tope", res.Estimation)
	}
	for _, p := range res.Passes {
		t.Logf("pasada %d: θ_D=%v mediana=%v excluido=%.1f%% ventanas6h=%d ventanas24h=%d marcas=%d",
			p.Pass, p.ThetaD, p.MedianB2, p.ExcludedRatio*100, p.Windows6h, p.Windows24h, p.MarkedHours)
	}
}
