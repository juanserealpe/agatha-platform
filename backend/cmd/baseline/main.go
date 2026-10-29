// Command baseline recalcula la línea base de un punto a demanda (Trigger =
// MANUAL). Es el instrumento de validación con el analista:
//
//	baseline -point 11556 -asof 2026-07-01T00:00:00Z [-dry-run] [-json]
//
// Con -dry-run imprime la versión calculada, las estadísticas por pasada y los
// periodos excluidos SIN persistir.
//
// Con -backfill carga el histórico de versiones que habrían existido desde el
// primer registro del dispositivo (o desde -from) hasta -asof, aplicando la
// política real de recálculo. Solo en un punto sin versiones:
//
//	baseline -point 11556 -backfill [-from 2026-06-07T05:00:00Z] [-asof ...] [-dry-run] [-json]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	baselineServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const runTimeout = 5 * time.Minute

func main() {
	log := logger.New("BaselineCLI", "Info")

	pointID := flag.Int64("point", 0, "measuring_point_id (obligatorio)")
	asOfRaw := flag.String("asof", "", "instante de corte RFC3339 en UTC (default: ahora)")
	dryRun := flag.Bool("dry-run", false, "calcula sin persistir")
	asJSON := flag.Bool("json", false, "imprime el resultado en JSON")
	backfill := flag.Bool("backfill", false, "carga el histórico de versiones (solo en un punto sin versiones)")
	fromRaw := flag.String("from", "", "con -backfill: inicio RFC3339 en UTC (default: primer registro de B2)")
	flag.Parse()

	if *pointID <= 0 {
		log.Error("-point es obligatorio y debe ser > 0")
		flag.Usage()
		os.Exit(2)
	}

	asOf := time.Now().UTC()
	if *asOfRaw != "" {
		parsed, err := time.Parse(time.RFC3339, *asOfRaw)
		if err != nil {
			log.Error("-asof inválido (RFC3339)", "value", *asOfRaw, "error", err)
			os.Exit(2)
		}
		asOf = parsed.UTC()
	}

	var from time.Time
	if *fromRaw != "" {
		if !*backfill {
			log.Error("-from solo aplica con -backfill")
			os.Exit(2)
		}
		parsed, err := time.Parse(time.RFC3339, *fromRaw)
		if err != nil {
			log.Error("-from inválido (RFC3339)", "value", *fromRaw, "error", err)
			os.Exit(2)
		}
		from = parsed.UTC()
	}

	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	module, err := bootstrap.NewBaselineModule(platform)
	if err != nil {
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	if *backfill {
		runBackfill(ctx, log, module, baselineServices.BackfillCommand{
			PointID: *pointID,
			From:    from,
			To:      asOf,
			DryRun:  *dryRun,
		}, *asJSON)
		return
	}

	result, err := module.Recalculator.Recalculate(ctx, baselineServices.RecalculateCommand{
		PointID: *pointID,
		AsOf:    asOf,
		Trigger: entities.TriggerManual,
		DryRun:  *dryRun,
	})
	if err != nil {
		log.Error("el recálculo falló", "error", err)
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

	printReport(result, asOf)
}

// runBackfill ejecuta UC-B4 e imprime una fila por versión.
func runBackfill(
	ctx context.Context,
	log *logger.Logger,
	module *bootstrap.BaselineModule,
	cmd baselineServices.BackfillCommand,
	asJSON bool,
) {
	result, err := module.Backfiller.Backfill(ctx, cmd)
	if err != nil {
		log.Error("la carga del histórico falló", "error", err)
		os.Exit(1)
	}

	if asJSON {
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Error("no fue posible serializar el resultado", "error", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	printBackfillReport(result)
}

func printBackfillReport(r baselineServices.BackfillResult) {
	fmt.Printf("Punto %d — histórico [%s, %s)\n", r.PointID, formatTime(r.From), formatTime(r.To))

	switch {
	case r.Created:
		fmt.Printf("Resultado: %d versiones creadas\n", len(r.Versions))
	case r.SkippedReason == baselineServices.SkippedDryRun:
		fmt.Printf("Resultado: DRY-RUN (%d versiones, no se persistió)\n", len(r.Versions))
	case r.SkippedReason == baselineServices.SkippedInsufficientHistory:
		fmt.Println("Resultado: INSUFFICIENT_HISTORY (no hay datos para ninguna versión)")
		return
	}

	fmt.Printf("\n  %-4s %-20s %-15s %-7s %8s %8s %7s %7s %7s  %s\n",
		"ver", "valid_from", "trigger", "conf", "mediana", "θ_D", "n", "v24h", "excl %", "flags")
	for _, v := range r.Versions {
		fmt.Printf("  %-4d %-20s %-15s %-7s %8g %8g %7d %7d %7.2f  %v\n",
			v.Version, formatTime(v.ValidFrom), v.Trigger, v.Confidence,
			v.Signal.MedianB2, v.Signal.ThetaD, v.Signal.NRecords, windows24h(v),
			v.Estimation.ExcludedRatio*100, v.Flags)
	}
}

// windows24h devuelve las ventanas de 24 h de la versión (0 si no tiene estadísticas de 24 h).
func windows24h(v entities.BaselineVersion) int {
	for _, st := range v.Components {
		if st.WindowHours == 24 {
			return st.NWindows
		}
	}
	return 0
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format(time.RFC3339)
}

func printReport(r baselineServices.RecalculationResult, asOf time.Time) {
	fmt.Printf("Punto %d — asOf %s\n", r.PointID, asOf.Format(time.RFC3339))

	if r.SkippedReason == baselineServices.SkippedInsufficientHistory {
		fmt.Println("Resultado: INSUFFICIENT_HISTORY (no se crea versión)")
		printPasses(r.Passes)
		return
	}

	switch {
	case r.Created:
		fmt.Printf("Resultado: versión %d creada (id %d)\n", r.Version.Version, r.Version.ID)
	case r.SkippedReason == baselineServices.SkippedDryRun:
		fmt.Println("Resultado: DRY-RUN (no se persistió)")
	default:
		fmt.Printf("Resultado: no creada (%s)\n", r.SkippedReason)
	}

	v := r.Version
	fmt.Printf("\nHistórico: [%s, %s)\n", v.HistoryFrom.Format(time.RFC3339), v.HistoryTo.Format(time.RFC3339))
	fmt.Printf("Confianza: %s   Flags: %v   Algoritmo: %s\n", v.Confidence, v.Flags, v.AlgorithmVersion)
	fmt.Printf("Estimación: %d pasadas, converge=%v, excluido=%.2f %%, tope=%v\n",
		v.Estimation.Passes, v.Estimation.Converged, v.Estimation.ExcludedRatio*100, v.Estimation.CapReached)

	s := v.Signal
	fmt.Println("\nSeñal")
	fmt.Printf("  mediana_B2=%g  θ_D=%g  Δmin=%g  n=%d  n_planitud=%d\n", s.MedianB2, s.ThetaD, s.DeltaMin, s.NRecords, s.NPlanitudRecords)
	fmt.Printf("  P90_Planitud=%s  min_elevado_B2=%s\n", optional(s.P90Planitud), optional(s.MinElevatedB2))

	fmt.Println("\nComponentes (MAD crudo)")
	fmt.Printf("  %-4s %-4s %12s %12s %9s\n", "h", "comp", "mediana", "MAD", "ventanas")
	for _, st := range v.Components {
		fmt.Printf("  %-4d %-4s %12.4f %12.4f %9d\n", st.WindowHours, st.Component, st.Median, st.MAD, st.NWindows)
	}

	printPasses(r.Passes)

	fmt.Printf("\nPeriodos excluidos (%d)\n", len(v.ExcludedPeriods))
	for _, p := range v.ExcludedPeriods {
		fmt.Printf("  [%s, %s)\n", p.From.Format(time.RFC3339), p.To.Format(time.RFC3339))
	}
}

func printPasses(passes []entities.PassReport) {
	fmt.Println("\nPasadas")
	fmt.Printf("  %-6s %-6s %10s %10s %10s %8s %8s %8s %6s\n", "pasada", "señal", "θ_D", "mediana", "excl %", "v6h", "v24h", "marcas", "tope")
	for _, p := range passes {
		fmt.Printf("  %-6d %-6v %10g %10g %10.2f %8d %8d %8d %6v\n",
			p.Pass, p.SignalOK, p.ThetaD, p.MedianB2, p.ExcludedRatio*100, p.Windows6h, p.Windows24h, p.MarkedHours, p.CapReached)
	}
}

func optional(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%g", *p)
}
