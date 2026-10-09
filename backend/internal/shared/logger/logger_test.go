package logger

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"
)

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })

	fn()
	return buf.String()
}

func TestLogger_TimestampIsBogotaWithOffset(t *testing.T) {
	out := captureOutput(t, func() {
		New("test", "Info").Info("hola")
	})

	// Un solo timestamp, al inicio, con offset explícito de Bogotá.
	pattern := regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} -05:00\] \[INFO\] \[test\] hola`)
	if !pattern.MatchString(out) {
		t.Fatalf("formato inesperado: %q", out)
	}
}

func TestLogger_NormalizesTimeFieldsToBogota(t *testing.T) {
	utc := time.Date(2026, 10, 9, 11, 45, 0, 0, time.UTC)

	out := captureOutput(t, func() {
		New("test", "Info").Info("evento", "at", utc, "ptr", &utc)
	})

	want := "2026-10-09T06:45:00-05:00"
	if strings.Count(out, want) != 2 {
		t.Fatalf("se esperaban ambos campos en hora Bogotá (%s): %q", want, out)
	}
	if strings.Contains(out, "UTC") {
		t.Fatalf("quedó un campo en UTC: %q", out)
	}
}
