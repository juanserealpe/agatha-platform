// Package mappers traduce entre las entidades del Rule Engine y sus filas.
package mappers

import (
	"encoding/json"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

func ToThresholdVersionModel(v entities.ThresholdVersion) models.RuleThresholdVersionModel {
	return models.RuleThresholdVersionModel{
		ID:                         v.ID,
		MeasuringPointID:           v.MeasuringPointID,
		EventType:                  string(v.EventType),
		Version:                    v.Version,
		Amarillo:                   v.Amarillo,
		Naranja:                    v.Naranja,
		Rojo:                       v.Rojo,
		MinHours:                   v.MinHours,
		EscalateAmarilloAfterHours: v.EscalateAmarilloAfterHours,
		EscalateNaranjaAfterHours:  v.EscalateNaranjaAfterHours,
		ExitMargin:                 v.ExitMargin,
		ExitHours:                  v.ExitHours,
		PersistenceMode:            string(v.PersistenceMode),
		PersistenceWindowHours:     v.PersistenceWindowHours,
		PersistenceMinHours:        v.PersistenceMinHours,
		ValidFrom:                  v.ValidFrom.UTC(),
		ValidTo:                    utcPtr(v.ValidTo),
		Reason:                     v.Reason,
		CreatedBy:                  v.CreatedBy,
		CreatedAt:                  v.CreatedAt.UTC(),
	}
}

func ToThresholdVersion(m models.RuleThresholdVersionModel) entities.ThresholdVersion {
	return entities.ThresholdVersion{
		ID:               m.ID,
		MeasuringPointID: m.MeasuringPointID,
		EventType:        entities.EventType(m.EventType),
		Version:          m.Version,
		ThresholdParams: entities.ThresholdParams{
			Amarillo:                   m.Amarillo,
			Naranja:                    m.Naranja,
			Rojo:                       m.Rojo,
			MinHours:                   m.MinHours,
			EscalateAmarilloAfterHours: m.EscalateAmarilloAfterHours,
			EscalateNaranjaAfterHours:  m.EscalateNaranjaAfterHours,
			ExitMargin:                 m.ExitMargin,
			ExitHours:                  m.ExitHours,
			PersistenceMode:            entities.PersistenceMode(m.PersistenceMode),
			PersistenceWindowHours:     m.PersistenceWindowHours,
			PersistenceMinHours:        m.PersistenceMinHours,
		}.Normalize(),
		ValidFrom: m.ValidFrom.UTC(),
		ValidTo:   utcPtr(m.ValidTo),
		Reason:    m.Reason,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt.UTC(),
	}
}

func ToThresholdVersions(rows []models.RuleThresholdVersionModel) []entities.ThresholdVersion {
	out := make([]entities.ThresholdVersion, len(rows))
	for i, m := range rows {
		out[i] = ToThresholdVersion(m)
	}
	return out
}

func ToRuleEvaluationModel(e entities.RuleEvaluation) models.RuleEvaluationModel {
	return models.RuleEvaluationModel{
		ID:                 e.ID,
		MeasuringPointID:   e.MeasuringPointID,
		EventType:          string(e.EventType),
		EvaluatedAt:        e.EvaluatedAt.UTC(),
		IndicatorValue:     e.IndicatorValue,
		RawLevel:           string(e.RawLevel),
		ComputedLevel:      string(e.ComputedLevel),
		Level:              string(e.Level),
		Rule:               string(e.Rule),
		InEpisode:          e.InEpisode,
		HoursAbove:         e.HoursAbove,
		HoursInLevel:       e.HoursInLevel,
		RunAmarillo:        e.RunAmarillo,
		RunNaranja:         e.RunNaranja,
		BelowRun:           e.BelowRun,
		ThresholdVersionID: e.ThresholdVersionID,
		LowConfidence:      e.LowConfidence,
		EpisodeEvent:       string(e.EpisodeEvent),
		Reasons:            reasonsJSON(e.Reasons),
		CreatedAt:          e.CreatedAt.UTC(),
	}
}

func ToRuleEvaluation(m models.RuleEvaluationModel) entities.RuleEvaluation {
	return entities.RuleEvaluation{
		ID:                 m.ID,
		MeasuringPointID:   m.MeasuringPointID,
		EventType:          entities.EventType(m.EventType),
		EvaluatedAt:        m.EvaluatedAt.UTC(),
		IndicatorValue:     m.IndicatorValue,
		RawLevel:           entities.Level(m.RawLevel),
		ComputedLevel:      entities.Level(m.ComputedLevel),
		Level:              entities.Level(m.Level),
		Rule:               entities.Rule(m.Rule),
		InEpisode:          m.InEpisode,
		HoursAbove:         m.HoursAbove,
		HoursInLevel:       m.HoursInLevel,
		RunAmarillo:        m.RunAmarillo,
		RunNaranja:         m.RunNaranja,
		BelowRun:           m.BelowRun,
		ThresholdVersionID: m.ThresholdVersionID,
		LowConfidence:      m.LowConfidence,
		EpisodeEvent:       entities.EpisodeEvent(m.EpisodeEvent),
		Reasons:            reasonsFromJSON(m.Reasons),
		CreatedAt:          m.CreatedAt.UTC(),
	}
}

// reasonsJSON serializa los motivos (siempre un arreglo, "[]" si no hay).
func reasonsJSON(reasons []entities.Reason) string {
	rows := make([]models.ReasonRow, len(reasons))
	for n, r := range reasons {
		rows[n] = models.ReasonRow{
			Code:  string(r.Code),
			Level: string(r.Level),
			Data: models.ReasonDataRow{
				I: r.I, Threshold: r.Threshold, Hours: r.Hours, From: string(r.From),
				WindowHours: r.WindowHours,
			},
		}
	}
	b, err := json.Marshal(rows)
	if err != nil { // no ocurre: solo strings y números
		return "[]"
	}
	return string(b)
}

// reasonsFromJSON lee la columna; vacía o "[]" ⇒ nil (como Assess sin motivos).
func reasonsFromJSON(raw string) []entities.Reason {
	var rows []models.ReasonRow
	if raw == "" || json.Unmarshal([]byte(raw), &rows) != nil || len(rows) == 0 {
		return nil
	}
	out := make([]entities.Reason, len(rows))
	for n, r := range rows {
		out[n] = entities.Reason{
			Code:        entities.ReasonCode(r.Code),
			Level:       entities.Level(r.Level),
			I:           r.Data.I,
			Threshold:   r.Data.Threshold,
			Hours:       r.Data.Hours,
			From:        entities.Level(r.Data.From),
			WindowHours: r.Data.WindowHours,
		}
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
