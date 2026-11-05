// Command rule_engine carga el histórico de interpretaciones del Rule Engine:
// lee de feature_evaluations las horas CALCULATED de 24 h y las interpreta, en
// orden y por punto, con EXACTAMENTE el mismo caso de uso que la evaluación en
// vivo, cada una con la versión de umbrales vigente en ella. NUNCA publica
// eventos. Solo completa las horas que faltan, así que es seguro repetirlo:
//
//	rule_engine -backfill [-point 11556] [-from 2026-07-09T00:00:00Z] [-to 2026-10-19T00:00:00Z]
//
// Con -rebuild borra antes las filas de rule_evaluations del rango/punto y las
// recalcula (bloqueado con AGATHA_ENV=production).
//
// Orden: primero processing -backfill (fuente), después rule_engine -backfill
// y recién entonces habilitar rule_engine en AGATHA_MODULES.
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
	ruleEngineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// runTimeout acota la corrida: ~4 consultas por hora interpretada.
const runTimeout = 2 * time.Hour

func main() {
	log := logger.New("RuleEngineCLI", "Info")

	backfill := flag.Bool("backfill", false, "interpreta el histórico de feature_evaluations (obligatorio)")
	pointID := flag.Int64("point", 0, "measuring_point_id (default: todos los puntos con horas calculadas)")
	fromRaw := flag.String("from", "", "inicio RFC3339 en UTC, inclusivo (default: primera hora)")
	toRaw := flag.String("to", "", "fin RFC3339 en UTC, exclusivo (default: última hora)")
	rebuild := flag.Bool("rebuild", false, "borra rule_evaluations del rango/punto y recalcula (no en production)")
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

	from := parseTime(log, "-from", *fromRaw)
	to := parseTime(log, "-to", *toRaw)

	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	module, err := bootstrap.NewRuleEngineModule(platform)
	if err != nil {
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	result, err := module.Backfiller.Backfill(ctx, ruleEngineServices.BackfillCommand{
		PointID: *pointID,
		From:    from,
		To:      to,
		Rebuild: *rebuild,
	})
	printReport(result)
	if err != nil {
		log.Error("el backfill del Rule Engine falló (lo guardado queda; repetirlo retoma)", "error", err)
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

func printReport(r ruleEngineServices.BackfillResult) {
	if r.Deleted > 0 {
		fmt.Printf("Rebuild: %d filas de rule_evaluations borradas\n", r.Deleted)
	}
	if len(r.Points) == 0 {
		fmt.Println("Resultado: no hay horas calculadas de 24 h en el rango (¿corriste processing -backfill?)")
		return
	}

	fmt.Printf("%-10s %6s %7s %6s %8s %6s %8s %8s %8s %8s %6s %6s  %s\n",
		"punto", "horas", "nuevas", "dupl.", "sin umbr", "tard.", "NORMAL", "AMARILLO", "NARANJA", "ROJO", "abren", "cierran", "rango")
	for _, p := range r.Points {
		printRow(fmt.Sprint(p.PointID), p)
	}
	if len(r.Points) > 1 {
		printRow("TOTAL", r.PointBackfill)
	}
}

func printRow(label string, p ruleEngineServices.PointBackfill) {
	fmt.Printf("%-10s %6d %7d %6d %8d %6d %8d %8d %8d %8d %6d %6d  [%s, %s]\n",
		label, p.Hours, p.Saved, p.Duplicates, p.NoThresholds, p.Late,
		p.HoursByLevel[entities.LevelNormal], p.HoursByLevel[entities.LevelAmarillo],
		p.HoursByLevel[entities.LevelNaranja], p.HoursByLevel[entities.LevelRojo],
		p.EpisodesOpened, p.EpisodesClosed, formatTime(p.First), formatTime(p.Last))
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.UTC().Format(time.RFC3339)
}
