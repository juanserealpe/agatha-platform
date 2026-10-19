// Package mappers traduce entre las entidades del Rule Engine y sus filas.
package mappers

import (
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
		},
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
		Level:              string(e.Level),
		Rule:               string(e.Rule),
		InEpisode:          e.InEpisode,
		HoursAbove:         e.HoursAbove,
		HoursInLevel:       e.HoursInLevel,
		ThresholdVersionID: e.ThresholdVersionID,
		LowConfidence:      e.LowConfidence,
		Emitted:            string(e.Emitted),
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
		Level:              entities.Level(m.Level),
		Rule:               entities.Rule(m.Rule),
		InEpisode:          m.InEpisode,
		HoursAbove:         m.HoursAbove,
		HoursInLevel:       m.HoursInLevel,
		ThresholdVersionID: m.ThresholdVersionID,
		LowConfidence:      m.LowConfidence,
		Emitted:            entities.Emission(m.Emitted),
		CreatedAt:          m.CreatedAt.UTC(),
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
