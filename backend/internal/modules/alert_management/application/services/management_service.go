package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/messages"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// Límites de texto (CA-2.3.2, RN-08).
const (
	maxOtherTextRunes   = 100
	maxObservationRunes = 500
)

// ActionOrigin es de dónde llega la acción (RN-10, sin inicio de sesión).
type ActionOrigin struct {
	Origin entities.Origin
	// DeviceIdentifier es obligatorio desde la app móvil.
	DeviceIdentifier string
	// ClientRecordID (UUID del celular): reenviarlo no duplica nada.
	ClientRecordID string
	// RecordedAt es la hora en el cliente (sin conexión); nil ⇒ la de recepción.
	RecordedAt *time.Time
}

// ClassifyCommand clasifica (paso 1).
type ClassifyCommand struct {
	AlertID        int64
	Classification entities.Classification
	Observation    string
	ActionOrigin
}

// LabelCommand etiqueta (paso 2).
type LabelCommand struct {
	AlertID     int64
	Label       entities.Label
	OtherText   string
	Observation string
	ActionOrigin
}

// ObservationCommand agrega una observación libre.
type ObservationCommand struct {
	AlertID int64
	Text    string
	ActionOrigin
}

// ManagementOutcome es cómo se resolvió la acción (la app lo muestra, CA-5.2.4).
type ManagementOutcome string

const (
	OutcomeSaved ManagementOutcome = "SAVED"
	// OutcomeConflict: ya estaba clasificada/etiquetada; se guardó como observación de conflicto.
	OutcomeConflict ManagementOutcome = "SAVED_AS_CONFLICT_OBSERVATION"
	// OutcomeAlreadyReceived: el client_record_id ya se había recibido.
	OutcomeAlreadyReceived ManagementOutcome = "ALREADY_RECEIVED"
)

// ManagementResult es la salida de la gestión.
type ManagementResult struct {
	Outcome ManagementOutcome
	Alert   entities.Alert
	// Existing es la gestión que prevaleció (solo con OutcomeConflict).
	Existing *entities.Management
}

// ManagementService guarda la gestión del operador. Cada paso queda fijo: el
// UPDATE solo afecta si el paso está vacío (y la BD lo impide con un
// trigger). Clasificar o etiquetar NO cambia el estado de la alerta.
type ManagementService struct {
	store ports.AlertStore
	clock ports.Clock
	log   *logger.Logger
}

func NewManagementService(store ports.AlertStore, clock ports.Clock, log *logger.Logger) *ManagementService {
	return &ManagementService{store: store, clock: clock, log: log}
}

// Classify (HU-2.2): solo con estado Sin clasificar.
func (s *ManagementService) Classify(ctx context.Context, cmd ClassifyCommand) (ManagementResult, error) {
	if !cmd.Classification.Valid() {
		return ManagementResult{}, domain.ErrInvalidClassification
	}
	if err := validateOrigin(cmd.ActionOrigin); err != nil {
		return ManagementResult{}, err
	}
	observation, err := optionalObservation(cmd.Observation)
	if err != nil {
		return ManagementResult{}, err
	}

	now := s.clock.Now().UTC()
	recordedAt := recordedAtOrNow(cmd.RecordedAt, now)
	m := entities.Management{
		Classification: cmd.Classification, ClassifiedAt: &now, ClassifiedOrigin: cmd.Origin,
		ClassifiedDevice: cmd.DeviceIdentifier, ClassifiedRecordedAt: &recordedAt, ClassifiedClientRecordID: cmd.ClientRecordID,
	}

	return s.run(ctx, cmd.AlertID, cmd.ClientRecordID, func(tx ports.AlertTx, alert entities.Alert) (ManagementResult, error) {
		updated, err := tx.Classify(ctx, alert.ID, m)
		if err != nil {
			return ManagementResult{}, err
		}
		if !updated {
			text := "Clasificación en conflicto: " + messages.ClassificationName(cmd.Classification) + " desde " + originText(cmd.ActionOrigin)
			return s.conflict(ctx, tx, alert, text, observation, entities.ObservationClassificationConflict, entities.StepClassification, cmd.ActionOrigin, now, recordedAt)
		}
		if err := s.observation(ctx, tx, alert.ID, observation, entities.StepClassification, cmd.ActionOrigin, now, recordedAt, cmd.ClientRecordID); err != nil {
			return ManagementResult{}, err
		}
		return s.saved(ctx, tx, alert.ID, messages.Key(messages.CodeClassified, ""), map[string]string{
			"clasificacion": messages.ClassificationName(cmd.Classification), "origen": originText(cmd.ActionOrigin),
		}, now)
	})
}

// Label (HU-2.3, HU-2.4): solo con estado Clasificada sin etiqueta y con la
// lista de su clasificación. OTRO exige other_text (1 a 100); con otra
// etiqueta el texto no se envía.
func (s *ManagementService) Label(ctx context.Context, cmd LabelCommand) (ManagementResult, error) {
	if err := validateOrigin(cmd.ActionOrigin); err != nil {
		return ManagementResult{}, err
	}
	otherText := strings.TrimSpace(cmd.OtherText)
	if cmd.Label == entities.LabelOther {
		if n := utf8.RuneCountInString(otherText); n < 1 || n > maxOtherTextRunes {
			return ManagementResult{}, domain.ErrOtherTextRequired
		}
	} else if otherText != "" {
		return ManagementResult{}, domain.ErrOtherTextRequired
	}
	observation, err := optionalObservation(cmd.Observation)
	if err != nil {
		return ManagementResult{}, err
	}

	now := s.clock.Now().UTC()
	recordedAt := recordedAtOrNow(cmd.RecordedAt, now)
	m := entities.Management{
		Label: cmd.Label, LabelOtherText: otherText, LabeledAt: &now, LabeledOrigin: cmd.Origin,
		LabeledDevice: cmd.DeviceIdentifier, LabeledRecordedAt: &recordedAt, LabeledClientRecordID: cmd.ClientRecordID,
	}

	return s.run(ctx, cmd.AlertID, cmd.ClientRecordID, func(tx ports.AlertTx, alert entities.Alert) (ManagementResult, error) {
		if !alert.Management.Classified() {
			return ManagementResult{}, domain.ErrNotClassified
		}
		if alert.Management.Label == "" && !cmd.Label.AllowedFor(alert.Management.Classification) {
			return ManagementResult{}, domain.ErrInvalidLabel
		}
		updated, err := tx.Label(ctx, alert.ID, m)
		if err != nil {
			return ManagementResult{}, err
		}
		if !updated {
			text := "Etiqueta en conflicto: " + labelText(cmd.Label, otherText) + " desde " + originText(cmd.ActionOrigin)
			return s.conflict(ctx, tx, alert, text, observation, entities.ObservationLabelConflict, entities.StepLabel, cmd.ActionOrigin, now, recordedAt)
		}
		if err := s.observation(ctx, tx, alert.ID, observation, entities.StepLabel, cmd.ActionOrigin, now, recordedAt, cmd.ClientRecordID); err != nil {
			return ManagementResult{}, err
		}
		return s.saved(ctx, tx, alert.ID, messages.Key(messages.CodeLabeled, ""), map[string]string{
			"etiqueta": labelText(cmd.Label, otherText), "origen": originText(cmd.ActionOrigin),
		}, now)
	})
}

// AddObservation (RN-08): solo se agregan, nunca reemplazan.
func (s *ManagementService) AddObservation(ctx context.Context, cmd ObservationCommand) (ManagementResult, error) {
	if err := validateOrigin(cmd.ActionOrigin); err != nil {
		return ManagementResult{}, err
	}
	text := strings.TrimSpace(cmd.Text)
	if n := utf8.RuneCountInString(text); n < 1 || n > maxObservationRunes {
		return ManagementResult{}, domain.ErrInvalidObservation
	}
	now := s.clock.Now().UTC()
	recordedAt := recordedAtOrNow(cmd.RecordedAt, now)

	return s.run(ctx, cmd.AlertID, cmd.ClientRecordID, func(tx ports.AlertTx, alert entities.Alert) (ManagementResult, error) {
		if err := s.observation(ctx, tx, alert.ID, text, entities.StepFree, cmd.ActionOrigin, now, recordedAt, cmd.ClientRecordID); err != nil {
			return ManagementResult{}, err
		}
		return ManagementResult{Outcome: OutcomeSaved, Alert: alert}, nil
	})
}

// run abre la TX: idempotencia por client_record_id y bloqueo de la alerta.
func (s *ManagementService) run(ctx context.Context, alertID int64, clientRecordID string,
	fn func(tx ports.AlertTx, alert entities.Alert) (ManagementResult, error),
) (ManagementResult, error) {
	if alertID <= 0 {
		return ManagementResult{}, domain.ErrInvalidAlertID
	}
	var result ManagementResult
	var appErr error
	err := s.store.Transaction(ctx, func(tx ports.AlertTx) error {
		alert, err := tx.GetAlert(ctx, alertID, true)
		if errors.Is(err, ports.ErrNotFound) {
			appErr = domain.ErrAlertNotFound
			return appErr
		}
		if err != nil {
			return err
		}
		if clientRecordID != "" {
			seen, err := tx.ClientRecordSeen(ctx, clientRecordID)
			if err != nil {
				return err
			}
			if seen {
				result = ManagementResult{Outcome: OutcomeAlreadyReceived, Alert: *alert}
				return nil
			}
		}
		result, err = fn(tx, *alert)
		if isAppError(err) {
			appErr = err
		}
		return err
	})
	if appErr != nil {
		return ManagementResult{}, appErr
	}
	if err != nil {
		s.log.Error("[ManagementService] Error guardando la gestión", "alert_id", alertID, "error", err)
		return ManagementResult{}, domain.ErrPersistingAlerts
	}
	return result, nil
}

func (s *ManagementService) conflict(ctx context.Context, tx ports.AlertTx, alert entities.Alert, text, observation string,
	kind entities.ObservationKind, step entities.ObservationStep, origin ActionOrigin, now, recordedAt time.Time,
) (ManagementResult, error) {
	if observation != "" {
		text += ". " + observation
	}
	if utf8.RuneCountInString(text) > maxObservationRunes {
		text = string([]rune(text)[:maxObservationRunes])
	}
	if _, _, err := tx.InsertObservation(ctx, entities.Observation{
		AlertID: alert.ID, Text: text, Kind: kind, Step: step, Origin: origin.Origin,
		DeviceIdentifier: origin.DeviceIdentifier, ClientRecordID: origin.ClientRecordID,
		RecordedAt: recordedAt, ReceivedAt: now,
	}); err != nil {
		return ManagementResult{}, err
	}
	existing := alert.Management
	s.log.Warn("[ManagementService] Gestión en conflicto: prevalece la primera", "alert", alert.Code, "kind", kind, "origin", origin.Origin)
	return ManagementResult{Outcome: OutcomeConflict, Alert: alert, Existing: &existing}, nil
}

func (s *ManagementService) observation(ctx context.Context, tx ports.AlertTx, alertID int64, text string, step entities.ObservationStep,
	origin ActionOrigin, now, recordedAt time.Time, clientRecordID string,
) error {
	if text == "" {
		return nil
	}
	_, _, err := tx.InsertObservation(ctx, entities.Observation{
		AlertID: alertID, Text: text, Kind: entities.ObservationFree, Step: step, Origin: origin.Origin,
		DeviceIdentifier: origin.DeviceIdentifier, ClientRecordID: clientRecordID, RecordedAt: recordedAt, ReceivedAt: now,
	})
	return err
}

// saved registra el mensaje de gestión en el color vigente (o el último) y devuelve la alerta actualizada.
func (s *ManagementService) saved(ctx context.Context, tx ports.AlertTx, alertID int64, key string, params map[string]string, now time.Time) (ManagementResult, error) {
	alert, err := tx.GetAlert(ctx, alertID, false)
	if err != nil {
		return ManagementResult{}, err
	}
	segment, err := tx.GetCurrentSegment(ctx, alertID)
	if err != nil {
		return ManagementResult{}, err
	}
	text, err := messages.Render(key, params)
	if err != nil {
		return ManagementResult{}, err
	}
	m := entities.Message{
		AlertID: alertID, EvaluatedAt: now, Kind: text.Kind, Code: text.Code, Level: alert.CurrentLevel,
		Title: text.Title, Body: text.Body, Params: params, CatalogVersion: messages.MessageCatalogVersion,
		PolicyVersionID: alert.PolicyVersionID, CreatedAt: now,
	}
	if segment != nil {
		m.SegmentID = segment.ID
	}
	if _, _, err := tx.InsertMessage(ctx, m); err != nil {
		return ManagementResult{}, err
	}
	return ManagementResult{Outcome: OutcomeSaved, Alert: *alert}, nil
}

func validateOrigin(o ActionOrigin) error {
	if !o.Origin.Valid() || (o.Origin == entities.OriginMobile && strings.TrimSpace(o.DeviceIdentifier) == "") {
		return domain.ErrInvalidOrigin
	}
	if o.ClientRecordID != "" {
		if _, err := uuid.Parse(o.ClientRecordID); err != nil {
			return domain.ErrInvalidClientRecordID
		}
	}
	return nil
}

func optionalObservation(text string) (string, error) {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > maxObservationRunes {
		return "", domain.ErrInvalidObservation
	}
	return text, nil
}

func recordedAtOrNow(t *time.Time, now time.Time) time.Time {
	if t == nil || t.IsZero() {
		return now
	}
	return t.UTC()
}

func originText(o ActionOrigin) string {
	if o.Origin == entities.OriginMobile {
		return o.Origin.DisplayName() + " (" + o.DeviceIdentifier + ")"
	}
	return o.Origin.DisplayName()
}

func labelText(l entities.Label, otherText string) string {
	if l == entities.LabelOther {
		return "Otro: " + otherText
	}
	if name := l.DisplayName(); name != "" {
		return name
	}
	return string(l)
}
