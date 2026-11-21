package services

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// defaultMotionChunkHours son las horas que se evalúan por cada lectura de
// raw_measurements: una semana de horas + la ventana de MotionWindowHours en una consulta.
const defaultMotionChunkHours = 168

// MotionPointRun es el resumen de un punto en una corrida (scheduler o backfill).
type MotionPointRun struct {
	PointID int64
	// WithoutHistory: el punto no tiene filas; en vivo se saltea (correr
	// rule_engine -backfill -motion).
	WithoutHistory bool
	// WithoutMeasurements: el backfill no encontró mediciones del punto.
	WithoutMeasurements bool
	Hours               int // horas recorridas
	Saved               int
	Duplicates          int
	NoThresholds        int
	NoCalculable        int
	Published           int
	Opened              int
	Cleared             int
	First, Last         *time.Time
	Err                 error
}

// MotionEvaluationService evalúa las horas de movimiento de un punto en orden
// ascendente: lee raw_measurements, corre MotionAssess con la versión vigente
// en cada hora y el estado de la fila anterior, guarda la fila y, después del
// commit, publica motion_evaluated (solo en vivo y con alerts_enabled). Solo
// orquesta e IO: la lógica vive en domain/services.MotionAssess.
type MotionEvaluationService struct {
	thresholds   repositories.MotionThresholdRepository
	evaluations  repositories.MotionEvaluationRepository
	thresholdUCs *MotionThresholdService
	source       ports.RawMeasurementSource
	publisher    ports.MotionEventPublisher // nil ⇒ no publica
	clock        ports.Clock
	log          *logger.Logger
	chunkHours   int

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func NewMotionEvaluationService(
	thresholds repositories.MotionThresholdRepository,
	evaluations repositories.MotionEvaluationRepository,
	thresholdUCs *MotionThresholdService,
	source ports.RawMeasurementSource,
	publisher ports.MotionEventPublisher,
	clock ports.Clock,
	log *logger.Logger,
) *MotionEvaluationService {
	return &MotionEvaluationService{
		thresholds: thresholds, evaluations: evaluations, thresholdUCs: thresholdUCs,
		source: source, publisher: publisher, clock: clock, log: log,
		chunkHours: defaultMotionChunkHours, locks: map[int64]*sync.Mutex{},
	}
}

// EvaluateLive evalúa las horas del punto desde su última fila + 1 h hasta el
// inicio de la hora actual: LIVE la última, CATCH_UP las anteriores. Publica.
// Un punto sin filas se saltea (WithoutHistory).
func (s *MotionEvaluationService) EvaluateLive(ctx context.Context, pointID int64) MotionPointRun {
	unlock := s.lock(pointID)
	defer unlock()

	run := MotionPointRun{PointID: pointID}
	last, err := s.evaluations.GetLastBefore(ctx, pointID, time.Time{})
	if err != nil {
		s.log.Error("[MotionEvaluationService] Error consultando la última hora evaluada", "measuring_point_id", pointID, "error", err)
		run.Err = domain.ErrConsultingMotionEvaluations
		return run
	}
	if last == nil {
		run.WithoutHistory = true
		return run
	}
	end := s.clock.Now().UTC().Truncate(time.Hour)
	trigger := func(t time.Time) entities.MotionTrigger {
		if t.Equal(end) {
			return entities.MotionTriggerLive
		}
		return entities.MotionTriggerCatchUp
	}
	s.evaluateRange(ctx, &run, last, last.EvaluatedAt.Add(time.Hour), end, trigger, true)
	return run
}

// evaluateRange evalúa las horas [from, to] (ambas incluidas) del punto. prev
// es la fila anterior a from (nil = sin historia). Debe llamarse con el lock
// del punto tomado. Un error detiene el punto: el orden importa.
func (s *MotionEvaluationService) evaluateRange(
	ctx context.Context,
	run *MotionPointRun,
	prev *entities.MotionEvaluation,
	from, to time.Time,
	trigger func(time.Time) entities.MotionTrigger,
	publish bool,
) {
	pointID := run.PointID
	var state entities.MotionState
	if prev != nil {
		state = prev.State
	}
	var version *entities.MotionThresholdVersion
	// Episodios cerrados antes de from (historial de motion-v2); se extiende
	// con cada CLEARED del tramo.
	past, err := s.evaluations.ListEpisodes(ctx, pointID, from)
	if err != nil {
		s.log.Error("[MotionEvaluationService] Error consultando los episodios anteriores", "measuring_point_id", pointID, "error", err)
		run.Err = domain.ErrConsultingMotionEvaluations
		return
	}

	for chunk := from; !chunk.After(to); chunk = chunk.Add(time.Duration(s.chunkHours) * time.Hour) {
		if ctx.Err() != nil {
			run.Err = ctx.Err()
			return
		}
		chunkEnd := chunk.Add(time.Duration(s.chunkHours-1) * time.Hour)
		if chunkEnd.After(to) {
			chunkEnd = to
		}
		winFrom, _ := entities.MotionWindow(chunk)
		records, err := s.source.ListRecords(ctx, pointID, winFrom, chunkEnd)
		if err != nil {
			s.log.Error("[MotionEvaluationService] Error leyendo raw_measurements",
				"measuring_point_id", pointID, "from", winFrom, "to", chunkEnd, "error", err)
			run.Err = domain.ErrConsultingRawMeasurements
			return
		}

		for t := chunk; !t.After(chunkEnd); t = t.Add(time.Hour) {
			run.Hours++
			if version == nil || !version.IsEffectiveAt(t) {
				if version, err = s.thresholds.GetEffectiveAt(ctx, pointID, t); err != nil {
					s.log.Error("[MotionEvaluationService] Error consultando la versión vigente",
						"measuring_point_id", pointID, "evaluated_at", t, "error", err)
					run.Err = domain.ErrConsultingThresholds
					return
				}
			}
			if version == nil {
				run.NoThresholds++
				continue
			}

			ev := domainServices.MotionAssess(windowOf(records, t), state, version.Params, t, past)
			ev.MeasuringPointID = pointID
			ev.ThresholdVersionID = version.ID
			ev.Trigger = trigger(t)
			ev.CreatedAt = s.clock.Now().UTC()

			id, inserted, err := s.evaluations.Save(ctx, ev)
			if err != nil {
				s.log.Error("[MotionEvaluationService] Error guardando la evaluación de movimiento",
					"measuring_point_id", pointID, "evaluated_at", t, "error", err)
				run.Err = domain.ErrPersistingMotionEvaluations
				return
			}
			if !inserted {
				// Otro proceso la guardó: se sigue desde SU fila.
				run.Duplicates++
				stored, err := s.evaluations.GetLastBefore(ctx, pointID, t.Add(time.Second))
				if err != nil || stored == nil || !stored.EvaluatedAt.Equal(t) {
					s.log.Error("[MotionEvaluationService] No se pudo leer la fila duplicada",
						"measuring_point_id", pointID, "evaluated_at", t, "error", err)
					run.Err = domain.ErrConsultingMotionEvaluations
					return
				}
				state = stored.State
				if ep, ok := entities.EpisodeFromCleared(*stored); ok {
					past = append(past, ep)
				}
				continue
			}
			ev.ID = id
			state = ev.State
			if ep, ok := entities.EpisodeFromCleared(ev); ok {
				past = append(past, ep)
			}
			s.count(run, ev)
			if publish && version.Params.AlertsEnabled {
				if s.publish(ctx, MotionEvaluatedFromRow(ev, version.Params, s.clock.Now().UTC())) {
					run.Published++
				}
			}
			if publish {
				s.logLive(ev, version)
			}
		}
	}
}

func (s *MotionEvaluationService) count(run *MotionPointRun, ev entities.MotionEvaluation) {
	run.Saved++
	if ev.Status == entities.MotionStatusNoCalculable {
		run.NoCalculable++
	}
	switch ev.EpisodeEvent {
	case entities.EpisodeEventOpened:
		run.Opened++
	case entities.EpisodeEventCleared:
		run.Cleared++
	}
	t := ev.EvaluatedAt
	if run.First == nil {
		run.First = &t
	}
	run.Last = &t
}

func (s *MotionEvaluationService) logLive(ev entities.MotionEvaluation, version *entities.MotionThresholdVersion) {
	codes := make([]string, len(ev.Reasons))
	for i, r := range ev.Reasons {
		codes[i] = string(r.Code)
	}
	fields := []any{
		"measuring_point_id", ev.MeasuringPointID, "evaluated_at", ev.EvaluatedAt, "status", ev.Status,
		"motion_state", ev.MotionState, "temp_state", ev.State.TempState, "episode_event", ev.EpisodeEvent,
		"reasons", codes, "trigger", ev.Trigger, "version", version.Version, "alerts_enabled", version.Params.AlertsEnabled,
	}
	if len(codes) > 0 || ev.EpisodeEvent != entities.EpisodeEventNone {
		s.log.Info("[MotionEvaluationService] Hora de movimiento evaluada", fields...)
		return
	}
	s.log.Debug("[MotionEvaluationService] Hora de movimiento evaluada", fields...)
}

// publish publica después del commit. Un fallo nunca falla la evaluación.
func (s *MotionEvaluationService) publish(ctx context.Context, event contract.MotionEvaluated) bool {
	if s.publisher == nil {
		return false
	}
	if err := s.publisher.PublishMotionEvaluated(ctx, event); err != nil {
		s.log.Warn("[MotionEvaluationService] No se pudo publicar motion_evaluated",
			"measuring_point_id", event.MeasuringPointID, "evaluated_at", event.EvaluatedAt, "error", err)
		return false
	}
	return true
}

func (s *MotionEvaluationService) lock(pointID int64) (unlock func()) {
	s.mu.Lock()
	l, ok := s.locks[pointID]
	if !ok {
		l = &sync.Mutex{}
		s.locks[pointID] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// windowOf devuelve los registros de la ventana de t de un tramo ordenado.
func windowOf(records []entities.RawRecord, t time.Time) []entities.RawRecord {
	from, to := entities.MotionWindow(t)
	i := sort.Search(len(records), func(k int) bool { return !records[k].Timestamp.Before(from) })
	j := sort.Search(len(records), func(k int) bool { return !records[k].Timestamp.Before(to) })
	return records[i:j]
}

// ceilHour es la primera hora en punto >= t.
func ceilHour(t time.Time) time.Time {
	h := t.UTC().Truncate(time.Hour)
	if h.Before(t) {
		h = h.Add(time.Hour)
	}
	return h
}

// MotionEvaluatedFromRow arma el evento v2 de una fila guardada (§7.2 de motion-v2).
func MotionEvaluatedFromRow(ev entities.MotionEvaluation, p entities.MotionParams, occurredAt time.Time) contract.MotionEvaluated {
	st := ev.State
	out := contract.MotionEvaluated{
		MotionEvaluationID: ev.ID,
		MeasuringPointID:   ev.MeasuringPointID,
		EventType:          contract.EventTypeMotion,
		EvaluatedAt:        ev.EvaluatedAt.UTC(),
		Status:             string(ev.Status),
		MotionState:        string(ev.MotionState),
		TempState:          string(st.TempState),
		TempMode:           string(p.TempMode),
		ComputedLevel:      string(ev.ComputedLevel),
		TableLevel:         string(ev.TableLevel),
		EpisodeKind:        string(ev.EpisodeKind),
		RulesHit:           contract.JSONArray(ev.RulesHit),
		InEpisode:          st.InEpisode,
		EpisodeEvent:       string(ev.EpisodeEvent),
		TempStable:         contract.TempStable{From: utc(ev.TempStableFrom()), To: utc(ev.TempStableTo())},
		Reasons:            make([]contract.MotionReason, len(ev.Reasons)),
		ThresholdVersionID: ev.ThresholdVersionID,
		OccurredAt:         occurredAt.UTC(),
		SchemaVersion:      contract.MotionSchemaVersion,
	}
	if st.InEpisode || ev.EpisodeEvent == entities.EpisodeEventCleared {
		out.EpisodeOpenedAt = utc(st.EpisodeOpenedAt)
	}
	if st.Displaced {
		out.Shift.StartedAt = utc(st.ShiftStartedAt)
		out.Shift.GapFrom, out.Shift.GapTo = utc(st.GapFrom), utc(st.GapTo)
		if ev.PitchNow != nil && st.RefPitch != nil {
			out.Shift.Pitch = round3Ptr(*ev.PitchNow - *st.RefPitch)
		}
		if ev.RollNow != nil && st.RefRoll != nil {
			out.Shift.Roll = round3Ptr(*ev.RollNow - *st.RefRoll)
		}
	}
	for i, r := range ev.Reasons {
		out.Reasons[i] = contract.MotionReason{Code: string(r.Code), Data: contract.JSONObject(r.Data())}
	}
	if ev.EpisodeEvent == entities.EpisodeEventOpened && ev.History != nil {
		out.History = contract.JSONObject(ev.History)
	}
	return out
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func round3Ptr(v float64) *float64 {
	r := math.Round(v*1000) / 1000
	return &r
}
