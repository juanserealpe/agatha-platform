// Command baseline recalcula la línea base de un punto a demanda (Trigger =
// MANUAL). Es el instrumento de validación con el analista:
//
//	baseline -point 11556 -asof 2026-07-01T00:00:00Z [-dry-run] [-json]
//
// Con -dry-run imprime la versión calculada, las estadísticas por pasada y los
// periodos excluidos SIN persistir.
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
		fmt.Printf("  [%s, %s)  pico I=%.4f\n", p.From.Format(time.RFC3339), p.To.Format(time.RFC3339), p.PeakI)
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
