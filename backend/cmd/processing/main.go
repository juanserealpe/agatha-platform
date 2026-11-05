// Command processing evalúa a demanda las horas pendientes de un punto
// (Trigger = MANUAL), igual que haría la evaluación horaria:
//
//	processing -point 11556
//
// Con -backfill carga el histórico de evaluaciones: todas las horas desde la
// primera con línea base (o desde -from) hasta la última hora lista (o
// -asof), cada una con la versión de línea base vigente en ella. Solo
// completa las horas que faltan, así que es seguro repetirlo. Requiere que el
// histórico de Baseline (baseline -backfill) esté cargado antes:
//
//	processing -point 11556 -backfill [-from 2026-07-09T00:00:00Z] [-asof ...] [-dry-run] [-json]
//
// Con -dry-run evalúa e imprime el resumen SIN persistir.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	processingServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const runTimeout = 10 * time.Minute

func main() {
	log := logger.New("ProcessingCLI", "Info")

	pointID := flag.Int64("point", 0, "measuring_point_id (obligatorio)")
	backfill := flag.Bool("backfill", false, "carga el histórico de evaluaciones (solo completa las horas que faltan)")
	fromRaw := flag.String("from", "", "con -backfill: inicio RFC3339 en UTC (default: primera hora con línea base)")
	asOfRaw := flag.String("asof", "", "con -backfill: fin RFC3339 en UTC (default: última hora lista)")
	dryRun := flag.Bool("dry-run", false, "con -backfill: evalúa sin persistir")
	asJSON := flag.Bool("json", false, "con -backfill: imprime todas las filas en JSON")
	flag.Parse()

	if *pointID <= 0 {
		log.Error("-point es obligatorio y debe ser > 0")
		flag.Usage()
		os.Exit(2)
	}
	if !*backfill && (*fromRaw != "" || *asOfRaw != "" || *dryRun || *asJSON) {
		log.Error("-from, -asof, -dry-run y -json solo aplican con -backfill")
		os.Exit(2)
	}

	from := parseTime(log, "-from", *fromRaw)
	to := parseTime(log, "-asof", *asOfRaw)

	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	module, err := bootstrap.NewProcessingModule(platform)
	if err != nil {
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	if !*backfill {
		result, err := module.Evaluator.EvaluatePending(ctx, processingServices.EvaluatePendingCommand{
			PointID: *pointID,
			Trigger: entities.TriggerManual,
		})
		if err != nil {
			log.Error("la evaluación falló", "error", err)
			os.Exit(1)
		}
		fmt.Printf("Punto %d — %d horas evaluadas, %d filas nuevas [%s, %s]\n",
			result.PointID, result.Planned, result.Inserted, formatTime(result.FirstSlot), formatTime(result.LastSlot))
		return
	}

	result, err := module.Evaluator.Backfill(ctx, processingServices.BackfillCommand{
		PointID: *pointID,
		From:    from,
		To:      to,
		DryRun:  *dryRun,
	})
	if err != nil {
		log.Error("la carga del histórico falló", "error", err)
		os.Exit(1)
	}

	if *asJSON {
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Error("no fue posible serializar el resultado", "error", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	printBackfillReport(result, *dryRun)
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

func printBackfillReport(r processingServices.BackfillResult, dryRun bool) {
	fmt.Printf("Punto %d — histórico [%s, %s]\n", r.PointID, formatTime(r.FirstSlot), formatTime(r.LastSlot))

	switch {
	case r.NoBaseline:
		fmt.Println("Resultado: el punto no tiene línea base (cargá antes baseline -backfill)")
		return
	case r.Hours == 0:
		fmt.Println("Resultado: no hay horas para evaluar en el rango")
		return
	case dryRun:
		fmt.Printf("Resultado: DRY-RUN (%d horas, %d filas, no se persistió)\n", r.Hours, len(r.Evaluations))
	default:
		fmt.Printf("Resultado: %d horas, %d filas, %d nuevas (%d ya existían)\n",
			r.Hours, len(r.Evaluations), r.Inserted, len(r.Evaluations)-r.Inserted)
	}

	printStatusSummary(r.Evaluations)
	printDailySummary(r.Evaluations)
}

// printStatusSummary cuenta, por ventana, las filas calculadas y los motivos de las que no.
func printStatusSummary(evals []entities.FeatureEvaluation) {
	counts := map[int]map[string]int{entities.Window24h: {}, entities.Window6h: {}}
	for _, e := range evals {
		key := string(e.Status)
		if e.Status == entities.StatusNoCalculable {
			key = string(e.NoCalcReason)
		}
		counts[e.WindowHours][key]++
	}

	fmt.Println("\nFilas por ventana")
	for _, hours := range []int{entities.Window24h, entities.Window6h} {
		keys := make([]string, 0, len(counts[hours]))
		for k := range counts[hours] {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Printf("  %2d h:", hours)
		for _, k := range keys {
			fmt.Printf("  %s=%d", k, counts[hours][k])
		}
		fmt.Println()
	}
}

// printDailySummary imprime, por día (UTC), cuántas horas se calcularon y el
// mínimo, el promedio y el máximo de I(24 h). Solo describe los valores: no
// los clasifica.
func printDailySummary(evals []entities.FeatureEvaluation) {
	type day struct {
		calculated int
		min, max   float64
		sum        float64
	}

	days := map[string]*day{}
	order := []string{}

	for _, e := range evals {
		if e.WindowHours != entities.Window24h {
			continue
		}

		key := e.EvaluatedAt.UTC().Format("2006-01-02")
		d, ok := days[key]
		if !ok {
			d = &day{}
			days[key] = d
			order = append(order, key)
		}

		if e.I == nil {
			continue
		}
		if d.calculated == 0 || *e.I < d.min {
			d.min = *e.I
		}
		if d.calculated == 0 || *e.I > d.max {
			d.max = *e.I
		}
		d.sum += *e.I
		d.calculated++
	}

	fmt.Println("\nI(24 h) por día (UTC)")
	fmt.Printf("  %-10s %6s %8s %8s %8s\n", "día", "horas", "I mín", "I medio", "I máx")
	for _, key := range order {
		d := days[key]
		if d.calculated == 0 {
			fmt.Printf("  %-10s %6d %8s %8s %8s\n", key, 0, "—", "—", "—")
			continue
		}
		fmt.Printf("  %-10s %6d %8.4f %8.4f %8.4f\n", key, d.calculated, d.min, d.sum/float64(d.calculated), d.max)
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.UTC().Format(time.RFC3339)
}
