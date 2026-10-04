// Package replay contiene el adaptador de salida de la herramienta de
// reproducción histórica (cmd/replay): un EvaluationRepository en memoria que
// escribe CSV en lugar de la BD.
package replay

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// Header son las columnas de feature_evaluations más mode y baseline_valid_from.
var Header = []string{
	"measuring_point_id", "window_hours", "evaluated_at", "window_start",
	"status", "no_calc_reason", "n_expected", "n_received", "n_valid", "coverage",
	"d", "p_hours", "ex", "c", "d_pl", "c_from_24h",
	"z_d", "z_p", "z_ex", "z_c", "s_d", "s_p", "s_ex", "s_c", "i",
	"low_confidence", "silent", "flags",
	"baseline_version_id", "baseline_version", "algorithm_version", "config_version",
	"trigger", "source_batch_seq", "input_max_ts", "created_at",
	"mode", "baseline_valid_from",
}

// CSVEvaluationSink implementa repositories.EvaluationRepository acumulando
// las filas en memoria. Nunca toca la BD.
type CSVEvaluationSink struct {
	mode string

	mu   sync.Mutex
	last *time.Time
	rows []entities.FeatureEvaluation
}

// NewCSVEvaluationSink crea el sink. GetLastEvaluatedAt devuelve inicialmente
// firstHour − 1 h, así el planificador empieza exactamente en firstHour.
func NewCSVEvaluationSink(mode string, firstHour time.Time) *CSVEvaluationSink {
	last := firstHour.UTC().Add(-time.Hour)
	return &CSVEvaluationSink{mode: mode, last: &last}
}

func (s *CSVEvaluationSink) GetLastEvaluatedAt(context.Context, int64) (*time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	last := *s.last
	return &last, nil
}

func (s *CSVEvaluationSink) SaveAll(_ context.Context, evals []entities.FeatureEvaluation) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range evals {
		if e.WindowHours == entities.Window24h && e.EvaluatedAt.After(*s.last) {
			at := e.EvaluatedAt
			s.last = &at
		}
	}
	s.rows = append(s.rows, evals...)

	return len(evals), nil
}

// Rows devuelve una copia de las filas acumuladas.
func (s *CSVEvaluationSink) Rows() []entities.FeatureEvaluation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]entities.FeatureEvaluation(nil), s.rows...)
}

// WriteCSV escribe el encabezado y una línea por fila (tiempos RFC3339 UTC,
// NULL = celda vacía, flags separados por "|").
func (s *CSVEvaluationSink) WriteCSV(w io.Writer) error {
	out := csv.NewWriter(w)

	if err := out.Write(Header); err != nil {
		return err
	}
	for _, e := range s.Rows() {
		if err := out.Write(s.record(e)); err != nil {
			return err
		}
	}

	out.Flush()
	return out.Error()
}

func (s *CSVEvaluationSink) record(e entities.FeatureEvaluation) []string {
	return []string{
		strconv.FormatInt(e.MeasuringPointID, 10),
		strconv.Itoa(e.WindowHours),
		ts(e.EvaluatedAt),
		ts(e.WindowStart),
		string(e.Status),
		string(e.NoCalcReason),
		strconv.Itoa(e.NExpected),
		strconv.Itoa(e.NReceived),
		strconv.Itoa(e.NValid),
		num(e.Coverage),
		optNum(e.D), optNum(e.PHours), optNum(e.Ex), optNum(e.C), optNum(e.DPl),
		strconv.FormatBool(e.CFrom24h),
		optNum(e.ZD), optNum(e.ZP), optNum(e.ZEx), optNum(e.ZC),
		optNum(e.SD), optNum(e.SP), optNum(e.SEx), optNum(e.SC),
		optNum(e.I),
		strconv.FormatBool(e.LowConfidence),
		strconv.FormatBool(e.Silent),
		strings.Join(e.Flags, "|"),
		optInt64(e.BaselineVersionID),
		optInt(e.BaselineVersion),
		e.AlgorithmVersion,
		e.ConfigVersion,
		string(e.Trigger),
		optInt64(e.SourceBatchSeq),
		optTs(e.InputMaxTs),
		ts(e.CreatedAt),
		s.mode,
		optTs(e.BaselineValidFrom),
	}
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func optNum(v *float64) string {
	if v == nil {
		return ""
	}
	return num(*v)
}

func optTs(t *time.Time) string {
	if t == nil {
		return ""
	}
	return ts(*t)
}

func optInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func optInt(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
