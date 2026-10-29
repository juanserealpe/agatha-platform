// Package memory implementa los ports de escritura de Alert Management en
// memoria (pruebas de los casos de uso). Cada transacción trabaja sobre una
// copia y solo se aplica si fn no devuelve error.
package memory

import (
	"context"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
)

type stateKey struct {
	pointID   int64
	eventType entities.EventType
}

type messageKey struct {
	alertID int64
	at      time.Time
	code    string
}

type data struct {
	states       map[stateKey]entities.ProcessingState
	alerts       map[int64]entities.Alert
	segments     []entities.Segment
	messages     []entities.Message
	messageKeys  map[messageKey]int64
	observations []entities.Observation
	seq          int64
}

func (d *data) clone() *data {
	c := &data{
		states:       maps.Clone(d.states),
		alerts:       maps.Clone(d.alerts),
		segments:     append([]entities.Segment(nil), d.segments...),
		messages:     append([]entities.Message(nil), d.messages...),
		messageKeys:  maps.Clone(d.messageKeys),
		observations: append([]entities.Observation(nil), d.observations...),
		seq:          d.seq,
	}
	return c
}

func (d *data) next() int64 {
	d.seq++
	return d.seq
}

// AlertStore es la base en memoria. FailNextTransaction simula un error de BD.
type AlertStore struct {
	mu   sync.Mutex
	d    *data
	fail error
}

func NewAlertStore() *AlertStore {
	return &AlertStore{d: &data{
		states:      map[stateKey]entities.ProcessingState{},
		alerts:      map[int64]entities.Alert{},
		messageKeys: map[messageKey]int64{},
		seq:         122, // el primer ID es 123 (ALR-000123), como en la especificación
	}}
}

// FailNextTransaction hace fallar la próxima transacción con err.
func (s *AlertStore) FailNextTransaction(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = err
}

func (s *AlertStore) Transaction(ctx context.Context, fn func(tx ports.AlertTx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		err := s.fail
		s.fail = nil
		return err
	}
	work := s.d.clone()
	if err := fn(&tx{d: work}); err != nil {
		return err
	}
	s.d = work
	return nil
}

// Lecturas para las pruebas (fuera de transacción).

func (s *AlertStore) Alerts() []entities.Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]entities.Alert, 0, len(s.d.alerts))
	for _, a := range s.d.alerts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *AlertStore) Alert(id int64) (entities.Alert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.d.alerts[id]
	return a, ok
}

func (s *AlertStore) Segments(alertID int64) []entities.Segment {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []entities.Segment
	for _, seg := range s.d.segments {
		if seg.AlertID == alertID {
			out = append(out, seg)
		}
	}
	return out
}

func (s *AlertStore) Messages() []entities.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]entities.Message(nil), s.d.messages...)
}

func (s *AlertStore) Observations() []entities.Observation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]entities.Observation(nil), s.d.observations...)
}

func (s *AlertStore) State(pointID int64, eventType entities.EventType) (entities.ProcessingState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.d.states[stateKey{pointID, eventType}]
	return st, ok
}

// SetManagement simula una gestión hecha por fuera (pruebas).
func (s *AlertStore) SetManagement(id int64, m entities.Management) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.d.alerts[id]
	a.Management = m
	s.d.alerts[id] = a
}

// ResetPoint borra los datos del punto (rebuild).
func (s *AlertStore) ResetPoint(_ context.Context, pointID int64, eventType entities.EventType) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted int64
	removed := map[int64]bool{}
	for id, a := range s.d.alerts {
		if a.MeasuringPointID == pointID && a.EventType == eventType {
			removed[id] = true
			delete(s.d.alerts, id)
			deleted++
		}
	}
	segs := s.d.segments[:0]
	for _, seg := range s.d.segments {
		if !removed[seg.AlertID] {
			segs = append(segs, seg)
		}
	}
	s.d.segments = segs
	msgs := s.d.messages[:0]
	for _, m := range s.d.messages {
		if removed[m.AlertID] {
			delete(s.d.messageKeys, messageKey{m.AlertID, m.EvaluatedAt, m.Code})
			continue
		}
		msgs = append(msgs, m)
	}
	s.d.messages = msgs
	obs := s.d.observations[:0]
	for _, o := range s.d.observations {
		if !removed[o.AlertID] {
			obs = append(obs, o)
		}
	}
	s.d.observations = obs
	delete(s.d.states, stateKey{pointID, eventType})
	return deleted, nil
}

type tx struct {
	d *data
}

func (t *tx) GetState(_ context.Context, pointID int64, eventType entities.EventType) (*entities.ProcessingState, error) {
	st, ok := t.d.states[stateKey{pointID, eventType}]
	if !ok {
		return nil, nil
	}
	return &st, nil
}

func (t *tx) SaveState(_ context.Context, st entities.ProcessingState) error {
	t.d.states[stateKey{st.MeasuringPointID, st.EventType}] = st
	return nil
}

func (t *tx) GetActive(_ context.Context, pointID int64, eventType entities.EventType) (*entities.Alert, error) {
	for _, a := range t.d.alerts {
		if a.MeasuringPointID == pointID && a.EventType == eventType && a.State == entities.AlertActive {
			return &a, nil
		}
	}
	return nil, nil
}

func (t *tx) GetAlert(_ context.Context, id int64, _ bool) (*entities.Alert, error) {
	a, ok := t.d.alerts[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return &a, nil
}

func (t *tx) InsertAlert(_ context.Context, a entities.Alert) (int64, error) {
	if a.State == entities.AlertActive {
		if active, _ := t.GetActive(context.Background(), a.MeasuringPointID, a.EventType); active != nil {
			return 0, ports.ErrConflict // índice único parcial: una ACTIVA por (punto, evento)
		}
	}
	a.ID = t.d.next()
	a.Code = entities.AlertCode(a.ID)
	t.d.alerts[a.ID] = a
	return a.ID, nil
}

func (t *tx) UpdateProcessing(_ context.Context, a entities.Alert) error {
	current, ok := t.d.alerts[a.ID]
	if !ok {
		return ports.ErrNotFound
	}
	a.Management = current.Management // la gestión no se toca desde el procesamiento
	a.Code = current.Code
	t.d.alerts[a.ID] = a
	return nil
}

func (t *tx) GetCurrentSegment(_ context.Context, alertID int64) (*entities.Segment, error) {
	var last *entities.Segment
	for i := range t.d.segments {
		if t.d.segments[i].AlertID == alertID {
			s := t.d.segments[i]
			last = &s
		}
	}
	return last, nil
}

func (t *tx) InsertSegment(_ context.Context, s entities.Segment) (int64, error) {
	seq := 0
	for _, existing := range t.d.segments {
		if existing.AlertID == s.AlertID && existing.Seq > seq {
			seq = existing.Seq
		}
	}
	s.ID = t.d.next()
	s.Seq = seq + 1
	t.d.segments = append(t.d.segments, s)
	return s.ID, nil
}

func (t *tx) CloseSegment(_ context.Context, segmentID int64, at time.Time) error {
	for i := range t.d.segments {
		if t.d.segments[i].ID == segmentID {
			t.d.segments[i].EndedAt = &at
		}
	}
	return nil
}

func (t *tx) InsertMessage(_ context.Context, m entities.Message) (int64, bool, error) {
	key := messageKey{m.AlertID, m.EvaluatedAt.UTC(), m.Code}
	if id, ok := t.d.messageKeys[key]; ok {
		return id, false, nil
	}
	m.ID = t.d.next()
	t.d.messageKeys[key] = m.ID
	t.d.messages = append(t.d.messages, m)
	return m.ID, true, nil
}

func (t *tx) Classify(_ context.Context, alertID int64, m entities.Management) (bool, error) {
	a, ok := t.d.alerts[alertID]
	if !ok {
		return false, ports.ErrNotFound
	}
	if a.Management.Classified() {
		return false, nil
	}
	a.Management.Status = entities.StatusClassifiedNoLabel
	a.Management.Classification = m.Classification
	a.Management.ClassifiedAt = m.ClassifiedAt
	a.Management.ClassifiedOrigin = m.ClassifiedOrigin
	a.Management.ClassifiedDevice = m.ClassifiedDevice
	a.Management.ClassifiedRecordedAt = m.ClassifiedRecordedAt
	a.Management.ClassifiedClientRecordID = m.ClassifiedClientRecordID
	t.d.alerts[alertID] = a
	return true, nil
}

func (t *tx) Label(_ context.Context, alertID int64, m entities.Management) (bool, error) {
	a, ok := t.d.alerts[alertID]
	if !ok {
		return false, ports.ErrNotFound
	}
	if !a.Management.Classified() || a.Management.Label != "" {
		return false, nil
	}
	a.Management.Status = entities.StatusClassifiedAndLabel
	a.Management.Label = m.Label
	a.Management.LabelOtherText = m.LabelOtherText
	a.Management.LabeledAt = m.LabeledAt
	a.Management.LabeledOrigin = m.LabeledOrigin
	a.Management.LabeledDevice = m.LabeledDevice
	a.Management.LabeledRecordedAt = m.LabeledRecordedAt
	a.Management.LabeledClientRecordID = m.LabeledClientRecordID
	t.d.alerts[alertID] = a
	return true, nil
}

func (t *tx) InsertObservation(_ context.Context, o entities.Observation) (int64, bool, error) {
	if o.ClientRecordID != "" {
		for _, existing := range t.d.observations {
			if existing.ClientRecordID == o.ClientRecordID {
				return existing.ID, false, nil
			}
		}
	}
	o.ID = t.d.next()
	t.d.observations = append(t.d.observations, o)
	return o.ID, true, nil
}

func (t *tx) ClientRecordSeen(_ context.Context, clientRecordID string) (bool, error) {
	for _, a := range t.d.alerts {
		if a.Management.ClassifiedClientRecordID == clientRecordID || a.Management.LabeledClientRecordID == clientRecordID {
			return true, nil
		}
	}
	for _, o := range t.d.observations {
		if o.ClientRecordID == clientRecordID {
			return true, nil
		}
	}
	return false, nil
}
