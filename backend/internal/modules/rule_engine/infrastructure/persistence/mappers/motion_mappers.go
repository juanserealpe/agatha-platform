package mappers

import (
	"encoding/json"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// ToMotionThresholdVersionModel / ToMotionThresholdVersion: motion_threshold_versions.
func ToMotionThresholdVersionModel(v entities.MotionThresholdVersion) models.MotionThresholdVersionModel {
	p := v.Params
	return models.MotionThresholdVersionModel{
		ID: v.ID, MeasuringPointID: v.MeasuringPointID, Version: v.Version,
		AlertsEnabled: p.AlertsEnabled, TempMode: string(p.TempMode),
		RecordsPerHour: p.RecordsPerHour, MinRecordsHour: p.MinRecordsHour,
		JumpDeg: p.JumpDeg, DerivaJumpDeg: p.DerivaJumpDeg,
		ShiftDeg: p.ShiftDeg, ShiftWindowHours: p.ShiftWindowHours, ShiftMinRecords: p.ShiftMinRecords,
		BaseFromHours: p.BaseFromHours, BaseToHours: p.BaseToHours, BaseMinRecords: p.BaseMinRecords,
		GapMinHours: p.GapMinHours, GapPreHours: p.GapPreHours, GapPostMinutes: p.GapPostMinutes,
		GapPreMinRecords: p.GapPreMinRecords, GapPostMinRecords: p.GapPostMinRecords,
		ReturnFraction: p.ReturnFraction, ShiftSuppressAfterReturnHours: p.ShiftSuppressAfterReturnHours,
		AbruptDeg: p.AbruptDeg, AbruptHours: p.AbruptHours, AbruptSuppressAfterJointHours: p.AbruptSuppressAfterJointHours,
		SlowDriftDeg24h: p.SlowDriftDeg24h,
		StillHours:      p.StillHours, StillMinCalculated: p.StillMinCalculated, StillRangeDeg: p.StillRangeDeg,
		TempFlatRate: p.TempFlatRate, TempFlatHours: p.TempFlatHours, TempFlatMinRates: p.TempFlatMinRates,
		TempFlowRate: p.TempFlowRate, TempFlowHours: p.TempFlowHours,
		StabilizeMaxHours:     p.StabilizeMaxHours,
		StillStableFirstHours: p.StillStableFirstHours, StillStableEveryHours: p.StillStableEveryHours,
		TrendDeg7d: p.TrendDeg7d,
		ValidFrom:  v.ValidFrom.UTC(), ValidTo: utcPtr(v.ValidTo),
		Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt.UTC(),
	}
}

func ToMotionThresholdVersion(m models.MotionThresholdVersionModel) entities.MotionThresholdVersion {
	return entities.MotionThresholdVersion{
		ID: m.ID, MeasuringPointID: m.MeasuringPointID, Version: m.Version,
		Params: entities.MotionParams{
			AlertsEnabled: m.AlertsEnabled, TempMode: entities.TempMode(m.TempMode),
			RecordsPerHour: m.RecordsPerHour, MinRecordsHour: m.MinRecordsHour,
			JumpDeg: m.JumpDeg, DerivaJumpDeg: m.DerivaJumpDeg,
			ShiftDeg: m.ShiftDeg, ShiftWindowHours: m.ShiftWindowHours, ShiftMinRecords: m.ShiftMinRecords,
			BaseFromHours: m.BaseFromHours, BaseToHours: m.BaseToHours, BaseMinRecords: m.BaseMinRecords,
			GapMinHours: m.GapMinHours, GapPreHours: m.GapPreHours, GapPostMinutes: m.GapPostMinutes,
			GapPreMinRecords: m.GapPreMinRecords, GapPostMinRecords: m.GapPostMinRecords,
			ReturnFraction: m.ReturnFraction, ShiftSuppressAfterReturnHours: m.ShiftSuppressAfterReturnHours,
			AbruptDeg: m.AbruptDeg, AbruptHours: m.AbruptHours, AbruptSuppressAfterJointHours: m.AbruptSuppressAfterJointHours,
			SlowDriftDeg24h: m.SlowDriftDeg24h,
			StillHours:      m.StillHours, StillMinCalculated: m.StillMinCalculated, StillRangeDeg: m.StillRangeDeg,
			TempFlatRate: m.TempFlatRate, TempFlatHours: m.TempFlatHours, TempFlatMinRates: m.TempFlatMinRates,
			TempFlowRate: m.TempFlowRate, TempFlowHours: m.TempFlowHours,
			StabilizeMaxHours:     m.StabilizeMaxHours,
			StillStableFirstHours: m.StillStableFirstHours, StillStableEveryHours: m.StillStableEveryHours,
			TrendDeg7d: m.TrendDeg7d,
		},
		ValidFrom: m.ValidFrom.UTC(), ValidTo: utcPtr(m.ValidTo),
		Reason: m.Reason, CreatedBy: m.CreatedBy, CreatedAt: m.CreatedAt.UTC(),
	}
}

func ToMotionThresholdVersions(rows []models.MotionThresholdVersionModel) []entities.MotionThresholdVersion {
	out := make([]entities.MotionThresholdVersion, len(rows))
	for i, m := range rows {
		out[i] = ToMotionThresholdVersion(m)
	}
	return out
}

// ToMotionEvaluationModel / ToMotionEvaluation: motion_evaluations. La vuelta
// es exacta: la hora siguiente lee el estado de esta fila.
func ToMotionEvaluationModel(e entities.MotionEvaluation) models.MotionEvaluationModel {
	s := e.State
	return models.MotionEvaluationModel{
		ID: e.ID, MeasuringPointID: e.MeasuringPointID, EvaluatedAt: e.EvaluatedAt.UTC(),
		Status: string(e.Status), NRecords: e.NRecords, Coverage: e.Coverage,

		PitchNow: e.PitchNow, RollNow: e.RollNow, TempNow: e.TempNow,
		DP1h: e.DP1h, DR1h: e.DR1h, BasePitch: e.BasePitch, BaseRoll: e.BaseRoll, DP6h: e.DP6h, DR6h: e.DR6h,
		ShiftRecords2h: e.ShiftRecords2h, ShiftSignHour: e.ShiftSignNow, DerivaMax1h: e.DerivaMax1h,
		Jump: e.Jump, Abrupt: e.Abrupt, DP24h: e.DP24h, DR24h: e.DR24h,
		SlowDriftFlag: e.SlowDriftFlag, TrendFlag: e.TrendFlag,
		TempRate: e.TempRate, TempRateMax6h: e.TempRateMax6h, TempFlat: e.TempFlat, TempFlow: e.TempFlow, Still: e.Still,
		GapEvalFrom: utcPtr(e.EvalGapFrom), GapEvalTo: utcPtr(e.EvalGapTo), DPGap: e.DPGap, DRGap: e.DRGap,

		Displaced: s.Displaced, RefPitch: s.RefPitch, RefRoll: s.RefRoll, ShiftSign: s.ShiftSign,
		ShiftStartedAt: utcPtr(s.ShiftStartedAt), ShiftLevelPitch: s.ShiftLevelPitch, ShiftLevelRoll: s.ShiftLevelRoll,
		GapFrom: utcPtr(s.GapFrom), GapTo: utcPtr(s.GapTo),
		LastJointAt: utcPtr(s.LastJointAt), LastReturnAt: utcPtr(s.LastReturnAt), ReturnStartedAt: utcPtr(s.ReturnStartedAt),
		TempState: optString(string(s.TempState)), TempSince: utcPtr(s.TempSince),
		TempStableFrom: utcPtr(s.TempStableFrom), TempStableTo: utcPtr(s.TempStableTo),
		StillStableMsgs: s.StillStableMsgs, InEpisode: s.InEpisode, EpisodeOpenedAt: utcPtr(s.EpisodeOpenedAt),
		AbruptActive: s.AbruptActive, PendingGaps: pendingGapsJSON(s.PendingGaps),

		MotionState: optString(string(e.MotionState)), EpisodeEvent: string(e.EpisodeEvent),
		ComputedLevel: string(e.ComputedLevel), Reasons: MotionReasonsJSON(e.Reasons),

		ThresholdVersionID: e.ThresholdVersionID, AlgorithmVersion: e.AlgorithmVersion,
		Trigger: string(e.Trigger), InputMaxTS: utcPtr(e.InputMaxTS), CreatedAt: e.CreatedAt.UTC(),
	}
}

func ToMotionEvaluation(m models.MotionEvaluationModel) entities.MotionEvaluation {
	return entities.MotionEvaluation{
		ID: m.ID, MeasuringPointID: m.MeasuringPointID, EvaluatedAt: m.EvaluatedAt.UTC(),
		Status: entities.MotionStatus(m.Status), NRecords: m.NRecords, Coverage: m.Coverage,

		PitchNow: m.PitchNow, RollNow: m.RollNow, TempNow: m.TempNow,
		DP1h: m.DP1h, DR1h: m.DR1h, BasePitch: m.BasePitch, BaseRoll: m.BaseRoll, DP6h: m.DP6h, DR6h: m.DR6h,
		ShiftRecords2h: m.ShiftRecords2h, ShiftSignNow: m.ShiftSignHour, DerivaMax1h: m.DerivaMax1h,
		Jump: m.Jump, Abrupt: m.Abrupt, DP24h: m.DP24h, DR24h: m.DR24h,
		SlowDriftFlag: m.SlowDriftFlag, TrendFlag: m.TrendFlag,
		TempRate: m.TempRate, TempRateMax6h: m.TempRateMax6h, TempFlat: m.TempFlat, TempFlow: m.TempFlow, Still: m.Still,
		EvalGapFrom: utcPtr(m.GapEvalFrom), EvalGapTo: utcPtr(m.GapEvalTo), DPGap: m.DPGap, DRGap: m.DRGap,

		State: entities.MotionState{
			Displaced: m.Displaced, RefPitch: m.RefPitch, RefRoll: m.RefRoll, ShiftSign: m.ShiftSign,
			ShiftStartedAt: utcPtr(m.ShiftStartedAt), ShiftLevelPitch: m.ShiftLevelPitch, ShiftLevelRoll: m.ShiftLevelRoll,
			GapFrom: utcPtr(m.GapFrom), GapTo: utcPtr(m.GapTo),
			LastJointAt: utcPtr(m.LastJointAt), LastReturnAt: utcPtr(m.LastReturnAt), ReturnStartedAt: utcPtr(m.ReturnStartedAt),
			TempState: entities.TempState(deref(m.TempState)), TempSince: utcPtr(m.TempSince),
			TempStableFrom: utcPtr(m.TempStableFrom), TempStableTo: utcPtr(m.TempStableTo),
			StillStableMsgs: m.StillStableMsgs, InEpisode: m.InEpisode, EpisodeOpenedAt: utcPtr(m.EpisodeOpenedAt),
			AbruptActive: m.AbruptActive, PendingGaps: pendingGapsFromJSON(m.PendingGaps),
		},

		MotionState: entities.MotionStateValue(deref(m.MotionState)), EpisodeEvent: entities.EpisodeEvent(m.EpisodeEvent),
		ComputedLevel: entities.Level(m.ComputedLevel), Reasons: MotionReasonsFromJSON(m.Reasons),

		ThresholdVersionID: m.ThresholdVersionID, AlgorithmVersion: m.AlgorithmVersion,
		Trigger: entities.MotionTrigger(m.Trigger), InputMaxTS: utcPtr(m.InputMaxTS), CreatedAt: m.CreatedAt.UTC(),
	}
}

// MotionReasonsJSON serializa los motivos como [{code, data}] (data = Reason.Data()).
func MotionReasonsJSON(reasons []entities.MotionReason) string {
	rows := make([]models.MotionReasonRow, len(reasons))
	for i, r := range reasons {
		rows[i] = models.MotionReasonRow{Code: string(r.Code), Data: r.Data()}
	}
	b, _ := json.Marshal(rows)
	return string(b)
}

// MotionReasonsFromJSON es la vuelta de MotionReasonsJSON.
func MotionReasonsFromJSON(raw string) []entities.MotionReason {
	var rows []models.MotionReasonRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]entities.MotionReason, len(rows))
	for i, row := range rows {
		d := row.Data
		out[i] = entities.MotionReason{
			Code:           entities.ReasonCode(row.Code),
			DP:             numPtr(d["dp"]),
			DR:             numPtr(d["dr"]),
			Deriva:         numPtr(d["deriva"]),
			StartedAt:      timePtr(d["started_at"]),
			GapFrom:        timePtr(d["gap_from"]),
			GapTo:          timePtr(d["gap_to"]),
			Axis:           str(d["axis"]),
			Delta:          num(d["delta"]),
			Hours:          int(num(d["hours"])),
			TempRateMax6h:  numPtr(d["temp_rate_max_6h"]),
			Rate:           num(d["rate"]),
			TempStableFrom: timePtr(d["temp_stable_from"]),
			TempStableTo:   timePtr(d["temp_stable_to"]),
		}
	}
	return out
}

func pendingGapsJSON(gaps []entities.PendingGap) string {
	rows := make([]models.PendingGapRow, len(gaps))
	for i, g := range gaps {
		rows[i] = models.PendingGapRow{From: g.From.UTC(), To: g.To.UTC()}
	}
	b, _ := json.Marshal(rows)
	return string(b)
}

func pendingGapsFromJSON(raw string) []entities.PendingGap {
	var rows []models.PendingGapRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]entities.PendingGap, len(rows))
	for i, r := range rows {
		out[i] = entities.PendingGap{From: r.From.UTC(), To: r.To.UTC()}
	}
	return out
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

func numPtr(v any) *float64 {
	f, ok := v.(float64)
	if !ok {
		return nil
	}
	return &f
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func timePtr(v any) *time.Time {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}
