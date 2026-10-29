// Command alert_management reconstruye el histórico de alertas: lee
// rule_evaluations (CALCULATED de 24 h) en orden y por punto y las procesa
// con EXACTAMENTE el mismo caso de uso que la evaluación en vivo, sin
// publicar avisos (source = BACKFILL). Las horas sin cálculo (sin fila) se procesan
// como NO_CALCULABLE. Solo completa lo que falta: es seguro repetirlo.
//
//	alert_management -backfill [-point 11556] [-from 2026-07-09T00:00:00Z] [-to 2026-10-27T00:00:00Z]
//
// Con -rebuild borra TODO lo del punto (alertas, colores, mensajes,
// observaciones, clasificaciones y estado) y lo reconstruye. Bloqueado con
// AGATHA_ENV=production.
//
// Orden: rule_engine -backfill → alert_management -backfill → habilitar
// alert_management en AGATHA_MODULES.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	alertServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const runTimeout = 2 * time.Hour

func main() {
	log := logger.New("AlertManagementCLI", "Info")

	backfill := flag.Bool("backfill", false, "reconstruye las alertas desde rule_evaluations sin publicar avisos (obligatorio)")
	pointID := flag.Int64("point", 0, "measuring_point_id (default: todos los puntos de rule_evaluations)")
	fromRaw := flag.String("from", "", "inicio RFC3339 en UTC, inclusivo (default: última hora procesada o el inicio)")
	toRaw := flag.String("to", "", "fin RFC3339 en UTC, exclusivo (default: última hora)")
	rebuild := flag.Bool("rebuild", false, "borra las alertas del punto y las reconstruye (no en production; pierde la gestión)")
	flag.Parse()

	if !*backfill {
		log.Error("-backfill es obligatorio (hoy es la única operación del comando)")
		flag.Usage()
		os.Exit(2)
	}
	if *pointID < 0 {
		log.Error("-point debe ser > 0")
		os.Exit(2)
	}
	from, to := parseTime(log, "-from", *fromRaw), parseTime(log, "-to", *toRaw)

	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}
	module, err := bootstrap.NewAlertManagementModule(platform)
	if err != nil {
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	result, err := module.Replay.Backfill(ctx, alertServices.BackfillCommand{PointID: *pointID, From: from, To: to, Rebuild: *rebuild})
	printReport(result)
	if err != nil {
		log.Error("el backfill de Alert Management falló (lo guardado queda; repetirlo retoma)", "error", err)
		os.Exit(1)
	}
}

func parseTime(log *logger.Logger, name, raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		log.Error(name+" inválido (RFC3339)", "value", raw, "error", err)
		os.Exit(2)
	}
	return parsed.UTC()
}

func printReport(r alertServices.BackfillResult) {
	if r.Deleted > 0 {
		fmt.Printf("Rebuild: %d alertas borradas\n", r.Deleted)
	}
	if len(r.Points) == 0 {
		fmt.Println("Resultado: no hay horas en rule_evaluations (¿corriste rule_engine -backfill?)")
		return
	}
	fmt.Printf("%-10s %7s %8s %8s %8s %10s\n", "punto", "horas", "mensajes", "abren", "reactiv.", "finalizan")
	row := func(label string, p alertServices.PointReplay) {
		fmt.Printf("%-10s %7d %8d %8d %8d %10d\n", label, p.Hours, p.Messages, p.Opened, p.Reactivated, p.Finished)
	}
	for _, p := range r.Points {
		row(fmt.Sprint(p.PointID), p)
	}
	if len(r.Points) > 1 {
		row("TOTAL", r.PointReplay)
	}
	fmt.Println("El backfill registra los mensajes pero no publica avisos (alert_management.alert_notification).")
}
