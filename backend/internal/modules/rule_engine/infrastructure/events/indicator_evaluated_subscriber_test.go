package events

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// captureLog redirige el log estándar (lo usa shared/logger) durante el test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

func TestHandle_LogsTheEvaluatedIndicator(t *testing.T) {
	out := captureLog(t)
	i := 0.73

	NewIndicatorEvaluatedSubscriber(logger.New("RuleEngine", "Info")).Handle(context.Background(), IndicatorEvaluatedNotice{
		MeasuringPointID: 11556,
		WindowHours:      6,
		EvaluatedAt:      time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Status:           "CALCULATED",
		I:                &i,
		Trigger:          "RECONCILER",
	})

	line := out.String()
	for _, want := range []string{"[RuleEngine]", "11556", "window_hours 6", "CALCULATED", "i 0.73", "RECONCILER"} {
		if !strings.Contains(line, want) {
			t.Errorf("el log no contiene %q: %s", want, line)
		}
	}
}

func TestHandle_LogsNoCalculableWithoutIndicator(t *testing.T) {
	out := captureLog(t)

	NewIndicatorEvaluatedSubscriber(logger.New("RuleEngine", "Info")).Handle(context.Background(), IndicatorEvaluatedNotice{
		MeasuringPointID: 11556,
		WindowHours:      24,
		EvaluatedAt:      time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Status:           "NO_CALCULABLE",
		NoCalcReason:     "LOW_COVERAGE",
	})

	line := out.String()
	if !strings.Contains(line, "LOW_COVERAGE") {
		t.Errorf("el log no contiene el motivo: %s", line)
	}
	// Un puntero nil no debe imprimirse como dirección ni como <nil>.
	if strings.Contains(line, "0x") || strings.Contains(line, "<nil>") {
		t.Errorf("el log imprime un puntero: %s", line)
	}
}
