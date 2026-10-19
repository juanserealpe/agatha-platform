package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// ClipTimeline devuelve los tramos de las versiones que se solapan con
// [from, to), recortados al rango y en el orden recibido (ascendente por
// valid_from):
//
//	solapa:      valid_from < to AND (valid_to IS NULL OR valid_to > from)
//	tramo.from = GREATEST(valid_from, from)
//	tramo.to   = LEAST(COALESCE(valid_to, to), to)
func ClipTimeline(versions []entities.ThresholdVersion, from, to time.Time) []entities.ThresholdSegment {
	segments := make([]entities.ThresholdSegment, 0, len(versions))

	for _, v := range versions {
		if !v.ValidFrom.Before(to) || (v.ValidTo != nil && !v.ValidTo.After(from)) {
			continue
		}

		start := v.ValidFrom
		if from.After(start) {
			start = from
		}
		end := to
		if v.ValidTo != nil && v.ValidTo.Before(end) {
			end = *v.ValidTo
		}

		segments = append(segments, entities.ThresholdSegment{From: start, To: end, Version: v})
	}

	return segments
}
