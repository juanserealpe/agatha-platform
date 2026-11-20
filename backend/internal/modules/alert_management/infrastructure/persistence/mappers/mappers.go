// Package mappers convierte entre las filas GORM y las entidades de Alert Management.
package mappers

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ToAlertModel convierte la alerta (sin code: es GENERATED).
func ToAlertModel(a entities.Alert) models.AlertModel {
	runtime, _ := json.Marshal(a.Runtime)
	m := a.Management
	leakDPl, leakDPlMin, leakConfirmed := leakColumns(a.LeakSignature)
	return models.AlertModel{
		LeakDPl: leakDPl, LeakDPlMin: leakDPlMin, LeakConfirmed: leakConfirmed,
		ID: a.ID, MeasuringPointID: a.MeasuringPointID, EventType: string(a.EventType), State: string(a.State),
		MotionState: strPtr(a.MotionState), TempState: strPtr(a.TempState),
		TempStableFrom: utcPtr(a.TempStableFrom), TempStableTo: utcPtr(a.TempStableTo),
		LastMotionEvaluatedAt: utcPtr(a.LastMotionEvaluatedAt),
		StartedAt:             a.StartedAt.UTC(), OpenedAt: a.OpenedAt.UTC(), EndedAt: a.EndedAt,
		ReactivationCount: a.ReactivationCount, LastReactivatedAt: a.LastReactivatedAt,
		CurrentLevel: string(a.CurrentLevel), MaxLevel: string(a.MaxLevel),
		OpeningIndicatorValue: a.OpeningIndicatorValue, LastIndicatorValue: a.LastIndicatorValue,
		MaxIndicatorValue: a.MaxIndicatorValue,
		LastEvaluatedAt:   a.LastEvaluatedAt.UTC(),
		HoursCalculated:   a.HoursCalculated, HoursNoData: a.HoursNoData, BelowLevelHours: a.BelowLevelHours,
		LowConfidenceAtOpen: a.LowConfidenceAtOpen, Runtime: string(runtime),
		PolicyVersionID: a.PolicyVersionID, ThresholdVersionIDAtOpen: a.ThresholdVersionIDAtOpen,
		OpenedByRuleEvaluationID: a.OpenedByRuleEvaluationID,
		ManagementStatus:         string(managementStatus(m)),
		Classification:           strPtr(string(m.Classification)), ClassifiedAt: m.ClassifiedAt,
		ClassifiedOrigin: strPtr(string(m.ClassifiedOrigin)), ClassifiedDevice: strPtr(m.ClassifiedDevice),
		ClassifiedRecordedAt: m.ClassifiedRecordedAt, ClassifiedClientRecordID: strPtr(m.ClassifiedClientRecordID),
		Label: strPtr(string(m.Label)), LabelOtherText: strPtr(m.LabelOtherText), LabeledAt: m.LabeledAt,
		LabeledOrigin: strPtr(string(m.LabeledOrigin)), LabeledDevice: strPtr(m.LabeledDevice),
		LabeledRecordedAt: m.LabeledRecordedAt, LabeledClientRecordID: strPtr(m.LabeledClientRecordID),
		Source: string(a.Source), CreatedAt: a.CreatedAt.UTC(),
	}
}

// leakColumns separa la firma de fuga en sus tres columnas (todas NULL sin firma).
func leakColumns(s *entities.LeakSignature) (dPl, dPlMin *float64, confirmed *bool) {
	if s == nil {
		return nil, nil, nil
	}
	min, ok := s.DPlMin, s.Confirmed
	return copyFloat(s.DPl), &min, &ok
}

func copyFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// leakSignature arma la firma desde las columnas; sin d_pl_min no hay firma.
func leakSignature(m models.AlertModel) *entities.LeakSignature {
	if m.LeakDPlMin == nil {
		return nil
	}
	return &entities.LeakSignature{DPl: copyFloat(m.LeakDPl), DPlMin: *m.LeakDPlMin, Confirmed: m.LeakConfirmed != nil && *m.LeakConfirmed}
}

func managementStatus(m entities.Management) entities.ManagementStatus {
	switch {
	case m.Classification == "":
		return entities.StatusUnclassified
	case m.Label == "":
		return entities.StatusClassifiedNoLabel
	default:
		return entities.StatusClassifiedAndLabel
	}
}

// ToAlert convierte la fila. Un runtime ilegible arranca vacío (solo afecta mensajes).
func ToAlert(m models.AlertModel) entities.Alert {
	var runtime entities.Runtime
	if m.Runtime != "" {
		_ = json.Unmarshal([]byte(m.Runtime), &runtime)
	}
	code := m.Code
	if code == "" {
		code = entities.AlertCode(m.ID)
	}
	return entities.Alert{
		ID: m.ID, Code: code, MeasuringPointID: m.MeasuringPointID, EventType: entities.EventType(m.EventType),
		MotionState: str(m.MotionState), TempState: str(m.TempState),
		TempStableFrom: utcPtr(m.TempStableFrom), TempStableTo: utcPtr(m.TempStableTo),
		LastMotionEvaluatedAt: utcPtr(m.LastMotionEvaluatedAt),
		State:                 entities.AlertState(m.State), StartedAt: m.StartedAt.UTC(), OpenedAt: m.OpenedAt.UTC(),
		EndedAt: utcPtr(m.EndedAt), ReactivationCount: m.ReactivationCount, LastReactivatedAt: utcPtr(m.LastReactivatedAt),
		CurrentLevel: entities.Level(m.CurrentLevel), MaxLevel: entities.Level(m.MaxLevel),
		OpeningIndicatorValue: m.OpeningIndicatorValue, LastIndicatorValue: m.LastIndicatorValue,
		MaxIndicatorValue: m.MaxIndicatorValue,
		LastEvaluatedAt:   m.LastEvaluatedAt.UTC(),
		HoursCalculated:   m.HoursCalculated, HoursNoData: m.HoursNoData, BelowLevelHours: m.BelowLevelHours,
		LowConfidenceAtOpen: m.LowConfidenceAtOpen, Runtime: runtime, LeakSignature: leakSignature(m),
		PolicyVersionID: m.PolicyVersionID, ThresholdVersionIDAtOpen: m.ThresholdVersionIDAtOpen,
		OpenedByRuleEvaluationID: m.OpenedByRuleEvaluationID,
		Management: entities.Management{
			Status:         entities.ManagementStatus(m.ManagementStatus),
			Classification: entities.Classification(str(m.Classification)), ClassifiedAt: utcPtr(m.ClassifiedAt),
			ClassifiedOrigin: entities.Origin(str(m.ClassifiedOrigin)), ClassifiedDevice: str(m.ClassifiedDevice),
			ClassifiedRecordedAt: utcPtr(m.ClassifiedRecordedAt), ClassifiedClientRecordID: str(m.ClassifiedClientRecordID),
			Label: entities.Label(str(m.Label)), LabelOtherText: str(m.LabelOtherText), LabeledAt: utcPtr(m.LabeledAt),
			LabeledOrigin: entities.Origin(str(m.LabeledOrigin)), LabeledDevice: str(m.LabeledDevice),
			LabeledRecordedAt: utcPtr(m.LabeledRecordedAt), LabeledClientRecordID: str(m.LabeledClientRecordID),
		},
		Source: entities.Source(m.Source), CreatedAt: m.CreatedAt.UTC(),
	}
}

// ProcessingColumns son las columnas que escribe el procesamiento horario
// (NUNCA las de gestión: las protege el trigger y las escribe otro caso de uso).
func ProcessingColumns(a entities.Alert) map[string]any {
	m := ToAlertModel(a)
	return map[string]any{
		"state": m.State, "ended_at": m.EndedAt, "reactivation_count": m.ReactivationCount,
		"last_reactivated_at": m.LastReactivatedAt, "current_level": m.CurrentLevel, "max_level": m.MaxLevel,
		"last_indicator_value": m.LastIndicatorValue, "max_indicator_value": m.MaxIndicatorValue,
		"last_evaluated_at": m.LastEvaluatedAt,
		"hours_calculated":  m.HoursCalculated, "hours_no_data": m.HoursNoData, "below_level_hours": m.BelowLevelHours,
		"runtime":      m.Runtime,
		"motion_state": m.MotionState, "temp_state": m.TempState,
		"temp_stable_from": m.TempStableFrom, "temp_stable_to": m.TempStableTo,
		"last_motion_evaluated_at": m.LastMotionEvaluatedAt,
		// La firma de fuga siempre se escribe (NULL incluido): describe la última hora.
		"leak_d_pl": m.LeakDPl, "leak_d_pl_min": m.LeakDPlMin, "leak_confirmed": m.LeakConfirmed,
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func ToSegmentModel(s entities.Segment) models.AlertLevelSegmentModel {
	return models.AlertLevelSegmentModel{
		ID: s.ID, AlertID: s.AlertID, Seq: s.Seq, OpenedBy: string(s.OpenedBy), Level: string(s.Level),
		StartedAt: s.StartedAt.UTC(), EndedAt: s.EndedAt,
	}
}

func ToSegment(m models.AlertLevelSegmentModel) entities.Segment {
	return entities.Segment{
		ID: m.ID, AlertID: m.AlertID, Seq: m.Seq, OpenedBy: entities.SegmentCause(m.OpenedBy),
		Level: entities.Level(m.Level), StartedAt: m.StartedAt.UTC(), EndedAt: utcPtr(m.EndedAt),
	}
}

func ToMessageModel(m entities.Message) models.AlertMessageModel {
	params, _ := json.Marshal(m.Params)
	if m.Params == nil {
		params = []byte("{}")
	}
	return models.AlertMessageModel{
		ID: m.ID, AlertID: m.AlertID, SegmentID: m.SegmentID, EvaluatedAt: m.EvaluatedAt.UTC(),
		Kind: string(m.Kind), Code: m.Code, Level: string(m.Level), IndicatorValue: m.IndicatorValue,
		Title: m.Title, Body: m.Body, Params: string(params), CatalogVersion: m.CatalogVersion,
		PolicyVersionID: m.PolicyVersionID, CreatedAt: m.CreatedAt.UTC(),
	}
}

func ToMessage(m models.AlertMessageModel) entities.Message {
	params := map[string]string{}
	_ = json.Unmarshal([]byte(m.Params), &params)
	return entities.Message{
		ID: m.ID, AlertID: m.AlertID, SegmentID: m.SegmentID, EvaluatedAt: m.EvaluatedAt.UTC(),
		Kind: entities.MessageKind(m.Kind), Code: m.Code, Level: entities.Level(m.Level),
		IndicatorValue: m.IndicatorValue, Title: m.Title, Body: m.Body, Params: params,
		CatalogVersion: m.CatalogVersion, PolicyVersionID: m.PolicyVersionID,
		CreatedAt: m.CreatedAt.UTC(),
	}
}

func ToObservationModel(o entities.Observation) models.AlertObservationModel {
	return models.AlertObservationModel{
		ID: o.ID, AlertID: o.AlertID, Text: o.Text, Kind: string(o.Kind), Step: string(o.Step),
		Origin: string(o.Origin), DeviceIdentifier: strPtr(o.DeviceIdentifier), ClientRecordID: strPtr(o.ClientRecordID),
		RecordedAt: o.RecordedAt.UTC(), ReceivedAt: o.ReceivedAt.UTC(),
	}
}

func ToObservation(m models.AlertObservationModel) entities.Observation {
	return entities.Observation{
		ID: m.ID, AlertID: m.AlertID, Text: m.Text, Kind: entities.ObservationKind(m.Kind),
		Step: entities.ObservationStep(m.Step), Origin: entities.Origin(m.Origin),
		DeviceIdentifier: str(m.DeviceIdentifier), ClientRecordID: str(m.ClientRecordID),
		RecordedAt: m.RecordedAt.UTC(), ReceivedAt: m.ReceivedAt.UTC(),
	}
}

func ToStateModel(s entities.ProcessingState) models.AlertProcessingStateModel {
	return models.AlertProcessingStateModel{
		MeasuringPointID: s.MeasuringPointID, EventType: string(s.EventType),
		LastEvaluatedAt: s.LastEvaluatedAt, PendingRunStartedAt: s.PendingRunStartedAt,
		LastAlertID: s.LastAlertID, LastStatus: s.LastStatus,
	}
}

func ToState(m models.AlertProcessingStateModel) entities.ProcessingState {
	return entities.ProcessingState{
		MeasuringPointID: m.MeasuringPointID, EventType: entities.EventType(m.EventType),
		LastEvaluatedAt: utcPtr(m.LastEvaluatedAt), PendingRunStartedAt: utcPtr(m.PendingRunStartedAt),
		LastAlertID: m.LastAlertID, LastStatus: m.LastStatus,
	}
}

func ToPolicyModel(p entities.Policy) models.AlertPolicyVersionModel {
	return models.AlertPolicyVersionModel{
		ID: p.ID, Version: p.Version, MeasuringPointID: p.MeasuringPointID, EventType: string(p.EventType),
		DowngradeHours:                        p.DowngradeHours,
		RojoRequiresClassificationToDowngrade: p.RojoRequiresClassificationToDowngrade,
		MergeWindowHours:                      p.MergeWindowHours,
		ReactivateOnlyIfUnclassified:          p.ReactivateOnlyIfUnclassified,
		InterventionSilentMaxLevel:            string(p.InterventionSilentMaxLevel),
		SummaryEveryHours:                     p.SummaryEveryHours,
		CountdownHours:                        p.CountdownHours,
		TrendLookbackHours:                    p.TrendLookbackHours,
		TrendDelta:                            p.TrendDelta,
		OscillationFlips:                      p.OscillationFlips,
		OscillationWindowHours:                p.OscillationWindowHours,
		LateNotificationMaxHours:              p.LateNotificationMaxHours,
		ValidFrom:                             p.ValidFrom.UTC(), ValidTo: p.ValidTo,
		Reason: p.Reason, CreatedBy: p.CreatedBy, CreatedAt: p.CreatedAt.UTC(),
	}
}

func ToPolicy(m models.AlertPolicyVersionModel) entities.Policy {
	return entities.Policy{
		ID: m.ID, Version: m.Version, MeasuringPointID: m.MeasuringPointID, EventType: entities.EventType(m.EventType),
		PolicyParams: entities.PolicyParams{
			DowngradeHours:                        m.DowngradeHours,
			RojoRequiresClassificationToDowngrade: m.RojoRequiresClassificationToDowngrade,
			MergeWindowHours:                      m.MergeWindowHours,
			ReactivateOnlyIfUnclassified:          m.ReactivateOnlyIfUnclassified,
			InterventionSilentMaxLevel:            entities.Level(m.InterventionSilentMaxLevel),
			SummaryEveryHours:                     m.SummaryEveryHours,
			CountdownHours:                        m.CountdownHours,
			TrendLookbackHours:                    m.TrendLookbackHours,
			TrendDelta:                            m.TrendDelta,
			OscillationFlips:                      m.OscillationFlips,
			OscillationWindowHours:                m.OscillationWindowHours,
			LateNotificationMaxHours:              m.LateNotificationMaxHours,
		},
		ValidFrom: m.ValidFrom.UTC(), ValidTo: utcPtr(m.ValidTo),
		Reason: m.Reason, CreatedBy: m.CreatedBy, CreatedAt: m.CreatedAt.UTC(),
	}
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

// ToLevelNotice reconstruye la hora del Rule Engine desde su fila y su versión
// de umbrales (las mismas reglas que rule_engine.LevelEvaluatedFromRow).
func ToLevelNotice(m models.RuleEvaluationReadModel) entities.LevelNotice {
	v := m.IndicatorValue
	n := entities.LevelNotice{
		RuleEvaluationID: m.ID, MeasuringPointID: m.MeasuringPointID, EventType: entities.EventType(m.EventType),
		EvaluatedAt: m.EvaluatedAt.UTC(), Status: entities.StatusCalculated, IndicatorValue: &v,
		RawLevel: entities.Level(m.RawLevel), ComputedLevel: entities.Level(m.ComputedLevel),
		EpisodeLevel: entities.Level(m.Level), InEpisode: m.InEpisode, EpisodeEvent: entities.EpisodeEvent(m.EpisodeEvent),
		Thresholds:         entities.Cuts{Amarillo: m.Amarillo, Naranja: m.Naranja, Rojo: m.Rojo},
		Exit:               entities.Cuts{Amarillo: round4(m.Amarillo - m.ExitMargin), Naranja: round4(m.Naranja - m.ExitMargin), Rojo: round4(m.Rojo - m.ExitMargin)},
		ThresholdVersionID: m.ThresholdVersionID, HoursAbove: m.HoursAbove,
		RunsAmarillo: m.RunAmarillo, RunsNaranja: m.RunNaranja, BelowRun: m.BelowRun, LowConfidence: m.LowConfidence,
		EscalationAmarillo: effectiveEscalation(m.EscalateAmarilloAfterHours, m),
		EscalationNaranja:  effectiveEscalation(m.EscalateNaranjaAfterHours, m),
		DPl:                copyFloat(m.DPl), DPlMin: copyFloat(m.DPlMin),
	}
	var reasons []models.ReasonReadRow
	if err := json.Unmarshal([]byte(m.Reasons), &reasons); err == nil {
		for _, r := range reasons {
			switch r.Code {
			case "LEAK_PERSISTENCE":
				n.PersistenceHours = r.Data.Hours
			case "NARANJA_NOT_CONFIRMED":
				n.LimitedFrom = entities.Level(r.Data.From)
			}
		}
	}
	return n
}

// effectiveEscalation: STRICT usa escalate_*; WINDOW, persistence_min_hours
// (un escalamiento NULL sigue desactivado).
func effectiveEscalation(escalate *int, m models.RuleEvaluationReadModel) *int {
	if escalate == nil {
		return nil
	}
	if strings.EqualFold(m.PersistenceMode, "WINDOW") && m.PersistenceMinHours != nil {
		v := *m.PersistenceMinHours
		return &v
	}
	v := *escalate
	return &v
}

// ToMotionNotice convierte una fila de motion_evaluations (con el
// alerts_enabled de su versión) en la vista propia de Alert Management.
func ToMotionNotice(r models.MotionEvaluationReadModel) entities.MotionNotice {
	n := entities.MotionNotice{
		MotionEvaluationID: r.ID, MeasuringPointID: r.MeasuringPointID, EvaluatedAt: r.EvaluatedAt.UTC(),
		Status: r.Status, MotionState: str(r.MotionState), TempState: str(r.TempState), TempMode: r.TempMode,
		ComputedLevel: entities.Level(r.ComputedLevel), InEpisode: r.InEpisode,
		EpisodeEvent:   entities.EpisodeEvent(r.EpisodeEvent),
		TempStableFrom: utcPtr(r.TempStableFrom), TempStableTo: utcPtr(r.TempStableTo),
		ThresholdVersionID: r.ThresholdVersionID, AlertsEnabled: r.AlertsEnabled,
	}
	if r.Displaced {
		n.ShiftStartedAt, n.GapFrom, n.GapTo = utcPtr(r.ShiftStartedAt), utcPtr(r.GapFrom), utcPtr(r.GapTo)
		if r.PitchNow != nil && r.RefPitch != nil {
			v := math.Round((*r.PitchNow-*r.RefPitch)*1000) / 1000
			n.ShiftPitch = &v
		}
		if r.RollNow != nil && r.RefRoll != nil {
			v := math.Round((*r.RollNow-*r.RefRoll)*1000) / 1000
			n.ShiftRoll = &v
		}
	}
	var rows []struct {
		Code string         `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.Reasons), &rows); err == nil {
		for _, row := range rows {
			n.Reasons = append(n.Reasons, entities.ParseMotionReason(row.Code, row.Data))
		}
	}
	return n
}
