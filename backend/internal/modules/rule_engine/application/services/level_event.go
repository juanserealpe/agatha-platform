package services

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// LevelEvaluatedFromRow arma la level_evaluated de una hora CALCULATED a
// partir de SOLO su fila y los parámetros de la versión con la que se
// interpretó (row.ThresholdVersionID). Es pura: el mismo evento se puede
// reconstruir más tarde desde rule_evaluations (p. ej., un reconciliador).
func LevelEvaluatedFromRow(row entities.RuleEvaluation, p entities.ThresholdParams, occurredAt time.Time) contract.LevelEvaluated {
	value := row.IndicatorValue

	reasons := make([]contract.Reason, len(row.Reasons))
	for n, r := range row.Reasons {
		reasons[n] = contract.Reason{
			Code:  string(r.Code),
			Level: string(r.Level),
			Data: contract.ReasonData{
				I:           r.I,
				Threshold:   r.Threshold,
				Hours:       r.Hours,
				From:        string(r.From),
				WindowHours: r.WindowHours,
				DPl:         copyFloat(r.DPl),
				DPlMin:      r.DPlMin,
			},
		}
	}

	return contract.LevelEvaluated{
		RuleEvaluationID: row.ID,
		MeasuringPointID: row.MeasuringPointID,
		EventType:        string(row.EventType),
		EvaluatedAt:      row.EvaluatedAt,
		Status:           contract.StatusCalculated,
		IndicatorValue:   &value,
		RawLevel:         string(row.RawLevel),
		ComputedLevel:    string(row.ComputedLevel),
		EpisodeLevel:     string(row.Level),
		InEpisode:        row.InEpisode,
		EpisodeEvent:     string(row.EpisodeEvent),
		Reasons:          reasons,
		Thresholds: contract.LevelCuts{
			Amarillo: p.CutFor(entities.LevelAmarillo),
			Naranja:  p.CutFor(entities.LevelNaranja),
			Rojo:     p.CutFor(entities.LevelRojo),
		},
		Exit: contract.LevelCuts{
			Amarillo: p.ExitFor(entities.LevelAmarillo),
			Naranja:  p.ExitFor(entities.LevelNaranja),
			Rojo:     p.ExitFor(entities.LevelRojo),
		},
		ThresholdVersionID: row.ThresholdVersionID,
		HoursAbove:         row.HoursAbove,
		Runs:               contract.LevelRuns{Amarillo: row.RunAmarillo, Naranja: row.RunNaranja},
		Escalation: contract.Escalation{
			AmarilloAfterHours: p.EscalationHours(entities.LevelAmarillo),
			NaranjaAfterHours:  p.EscalationHours(entities.LevelNaranja),
		},
		BelowRun:        row.BelowRun,
		DPl:             copyFloat(row.DPl),
		DPlMin:          copyFloat(p.DPlMin),
		PersistenceMode: string(p.Normalize().PersistenceMode),
		LowConfidence:   row.LowConfidence,
		OccurredAt:      occurredAt,
		SchemaVersion:   contract.SchemaVersion,
	}
}

func copyFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
