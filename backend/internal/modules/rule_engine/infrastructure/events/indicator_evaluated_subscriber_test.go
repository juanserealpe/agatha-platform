package events

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
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

type fakeEvaluator struct {
	received []services.IndicatorEvaluatedNotice
	err      error
}

func (f *fakeEvaluator) EvaluateIndicator(_ context.Context, n services.IndicatorEvaluatedNotice) (services.EvaluationResult, error) {
	f.received = append(f.received, n)
	return services.EvaluationResult{}, f.err
}

func notice() services.IndicatorEvaluatedNotice {
	i := 0.73
	return services.IndicatorEvaluatedNotice{
		MeasuringPointID: 11556,
		WindowHours:      24,
		EvaluatedAt:      time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Status:           "CALCULATED",
		I:                &i,
		Trigger:          "RECONCILER",
	}
}

func TestHandle_DelegatesToTheUseCase(t *testing.T) {
	evaluator := &fakeEvaluator{}

	NewIndicatorEvaluatedSubscriber(evaluator, logger.New("RuleEngine", "Info")).Handle(context.Background(), notice())

	if len(evaluator.received) != 1 {
		t.Fatalf("recibidos = %d, se esperaba 1", len(evaluator.received))
	}
	got, want := evaluator.received[0], notice()
	if got.MeasuringPointID != want.MeasuringPointID || got.WindowHours != want.WindowHours ||
		!got.EvaluatedAt.Equal(want.EvaluatedAt) || got.I == nil || *got.I != *want.I || got.Trigger != want.Trigger {
		t.Fatalf("notice entregado = %+v", got)
	}
}

func TestHandle_LogsUseCaseErrorsWithTheIndicator(t *testing.T) {
	out := captureLog(t)
	evaluator := &fakeEvaluator{err: domain.ErrPersistingRuleEvaluations}

	NewIndicatorEvaluatedSubscriber(evaluator, logger.New("RuleEngine", "Info")).Handle(context.Background(), notice())

	line := out.String()
	for _, want := range []string{"[ERROR]", "[RuleEngine]", "11556", "i 0.73", "RECONCILER", domain.ErrPersistingRuleEvaluations.Message} {
		if !strings.Contains(line, want) {
			t.Errorf("el log no contiene %q: %s", want, line)
		}
	}
}

func TestHandle_NoCalculableIsLoggedWithoutPointers(t *testing.T) {
	out := captureLog(t)
	n := notice()
	n.Status, n.NoCalcReason, n.I = "NO_CALCULABLE", "LOW_COVERAGE", nil

	NewIndicatorEvaluatedSubscriber(&fakeEvaluator{err: errors.New("x")}, logger.New("RuleEngine", "Debug")).Handle(context.Background(), n)

	line := out.String()
	if !strings.Contains(line, "LOW_COVERAGE") {
		t.Errorf("el log no contiene el motivo: %s", line)
	}
	// Un puntero nil no debe imprimirse como dirección ni como <nil>.
	if strings.Contains(line, "0x") || strings.Contains(line, "<nil>") {
		t.Errorf("el log imprime un puntero: %s", line)
	}
}
