package bootstrap

import (
	"testing"
	"time"

	ingestionContract "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/contract"
)

func TestToMeasurementsIngestedNotice(t *testing.T) {
	from := time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 28, 15, 10, 0, 0, time.UTC)

	notice := toMeasurementsIngestedNotice(ingestionContract.MeasurementsIngested{
		MeasuringPointID: 11556,
		Source:           "dataos",
		From:             from,
		To:               to,
		Watermark:        to,
		BatchSeq:         42,
		InsertedRecords:  3,
		OccurredAt:       time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC),
		SchemaVersion:    ingestionContract.SchemaVersion,
	})

	if notice.MeasuringPointID != 11556 || notice.InsertedRecords != 3 {
		t.Fatalf("notice = %+v", notice)
	}
	if !notice.Watermark.Equal(to) {
		t.Fatalf("Watermark = %v, se esperaba %v", notice.Watermark, to)
	}
	if notice.BatchSeq == nil || *notice.BatchSeq != 42 {
		t.Fatalf("BatchSeq = %v, se esperaba 42", notice.BatchSeq)
	}
	if notice.From == nil || !notice.From.Equal(from) {
		t.Fatalf("From = %v, se esperaba %v", notice.From, from)
	}
}

// Un evento sin datos de lote (zero values) no debe inventar cursores: Processing
// deriva la hora lista de la BD.
func TestToMeasurementsIngestedNotice_ZeroValuesStayEmpty(t *testing.T) {
	notice := toMeasurementsIngestedNotice(ingestionContract.MeasurementsIngested{MeasuringPointID: 11556})

	if !notice.Watermark.IsZero() || notice.BatchSeq != nil || notice.From != nil {
		t.Fatalf("notice = %+v, se esperaban campos vacíos", notice)
	}
}
