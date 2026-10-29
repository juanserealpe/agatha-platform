package services

import (
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

func at(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }

func version(n int, from time.Time, to *time.Time) entities.ThresholdVersion {
	return entities.ThresholdVersion{Version: n, ValidFrom: from, ValidTo: to}
}

func TestClipTimeline_TrimsBothEndsWithSeveralVersions(t *testing.T) {
	d5, d10 := at(5), at(10)
	versions := []entities.ThresholdVersion{
		version(1, at(1), &d5),
		version(2, d5, &d10),
		version(3, d10, nil), // vigente
	}

	got := ClipTimeline(versions, at(3), at(12))

	want := []struct {
		v        int
		from, to time.Time
	}{
		{1, at(3), at(5)},   // recortado al inicio: GREATEST(valid_from, from)
		{2, at(5), at(10)},  // intacto
		{3, at(10), at(12)}, // vigente recortada al final: LEAST(COALESCE(valid_to, to), to)
	}

	if len(got) != len(want) {
		t.Fatalf("tramos = %d, se esperaban %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Version.Version != w.v || !got[i].From.Equal(w.from) || !got[i].To.Equal(w.to) {
			t.Errorf("tramo %d = v%d [%v, %v), se esperaba v%d [%v, %v)",
				i, got[i].Version.Version, got[i].From, got[i].To, w.v, w.from, w.to)
		}
	}
}

func TestClipTimeline_DropsVersionsOutsideTheRange(t *testing.T) {
	d5, d10 := at(5), at(10)
	versions := []entities.ThresholdVersion{
		version(1, at(1), &d5), // termina justo donde empieza el rango: no se solapa
		version(2, d5, &d10),
		version(3, d10, nil), // empieza justo donde termina el rango: no se solapa
	}

	got := ClipTimeline(versions, at(5), at(10))
	if len(got) != 1 || got[0].Version.Version != 2 {
		t.Fatalf("tramos = %+v, se esperaba solo la v2", got)
	}
}

func TestClipTimeline_RangeInsideOneVersion(t *testing.T) {
	got := ClipTimeline([]entities.ThresholdVersion{version(1, at(1), nil)}, at(3), at(4))
	if len(got) != 1 || !got[0].From.Equal(at(3)) || !got[0].To.Equal(at(4)) {
		t.Fatalf("tramos = %+v", got)
	}
}
