package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

// ToFeatureEvaluationModel convierte la entidad en fila. BaselineValidFrom no
// se persiste (solo lo usa el CSV del replay).
func ToFeatureEvaluationModel(e entities.FeatureEvaluation) models.FeatureEvaluationModel {
	m := models.FeatureEvaluationModel{
		MeasuringPointID:  e.MeasuringPointID,
		WindowHours:       int16(e.WindowHours),
		EvaluatedAt:       e.EvaluatedAt.UTC(),
		WindowStart:       e.WindowStart.UTC(),
		Status:            string(e.Status),
		NExpected:         int16(e.NExpected),
		NReceived:         int16(e.NReceived),
		NValid:            int16(e.NValid),
		Coverage:          e.Coverage,
		D:                 e.D,
		PHours:            e.PHours,
		Ex:                e.Ex,
		C:                 e.C,
		DPl:               e.DPl,
		CFrom24h:          e.CFrom24h,
		ZD:                e.ZD,
		ZP:                e.ZP,
		ZEx:               e.ZEx,
		ZC:                e.ZC,
		SD:                e.SD,
		SP:                e.SP,
		SEx:               e.SEx,
		SC:                e.SC,
		I:                 e.I,
		LowConfidence:     e.LowConfidence,
		Silent:            e.Silent,
		Flags:             models.TextArray(nonNil(e.Flags)),
		BaselineVersionID: e.BaselineVersionID,
		BaselineVersion:   e.BaselineVersion,
		AlgorithmVersion:  e.AlgorithmVersion,
		ConfigVersion:     e.ConfigVersion,
		Trigger:           string(e.Trigger),
		SourceBatchSeq:    e.SourceBatchSeq,
		InputMaxTs:        e.InputMaxTs,
		CreatedAt:         e.CreatedAt.UTC(),
	}

	if e.NoCalcReason != entities.ReasonNone {
		reason := string(e.NoCalcReason)
		m.NoCalcReason = &reason
	}

	return m
}

// ToFeatureEvaluation convierte una fila en entidad.
func ToFeatureEvaluation(m models.FeatureEvaluationModel) entities.FeatureEvaluation {
	e := entities.FeatureEvaluation{
		MeasuringPointID:  m.MeasuringPointID,
		WindowHours:       int(m.WindowHours),
		EvaluatedAt:       m.EvaluatedAt.UTC(),
		WindowStart:       m.WindowStart.UTC(),
		Status:            entities.EvaluationStatus(m.Status),
		NExpected:         int(m.NExpected),
		NReceived:         int(m.NReceived),
		NValid:            int(m.NValid),
		Coverage:          m.Coverage,
		D:                 m.D,
		PHours:            m.PHours,
		Ex:                m.Ex,
		C:                 m.C,
		DPl:               m.DPl,
		CFrom24h:          m.CFrom24h,
		ZD:                m.ZD,
		ZP:                m.ZP,
		ZEx:               m.ZEx,
		ZC:                m.ZC,
		SD:                m.SD,
		SP:                m.SP,
		SEx:               m.SEx,
		SC:                m.SC,
		I:                 m.I,
		LowConfidence:     m.LowConfidence,
		Silent:            m.Silent,
		Flags:             nonNil(m.Flags),
		BaselineVersionID: m.BaselineVersionID,
		BaselineVersion:   m.BaselineVersion,
		AlgorithmVersion:  m.AlgorithmVersion,
		ConfigVersion:     m.ConfigVersion,
		Trigger:           entities.Trigger(m.Trigger),
		SourceBatchSeq:    m.SourceBatchSeq,
		InputMaxTs:        m.InputMaxTs,
		CreatedAt:         m.CreatedAt.UTC(),
	}

	if m.NoCalcReason != nil {
		e.NoCalcReason = entities.NoCalcReason(*m.NoCalcReason)
	}

	return e
}

func nonNil(flags []string) []string {
	if flags == nil {
		return []string{}
	}
	return flags
}
