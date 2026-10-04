// Command replay evalúa el pasado hora por hora con el MISMO código de
// producción de Processing, sin escribir en la base de datos, para calibrar
// con el analista:
//
//	replay -point 11556 -from 2026-07-08T00:00:00Z -to 2026-10-03T00:00:00Z \
//	       -mode asof|fixed [-fixed-asof <RFC3339>] -out replay_11556.csv
//
// -mode asof: en cada hora, la línea base que habría estado vigente (simulada).
// -mode fixed: una sola línea base calculada en -fixed-asof (default: -to).
//
// La conexión se abre con default_transaction_read_only=on: PostgreSQL
// rechaza cualquier escritura.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

const runTimeout = 30 * time.Minute

func main() {
	log := logger.New("ReplayCLI", "Info")

	pointID := flag.Int64("point", 0, "measuring_point_id (obligatorio)")
	fromRaw := flag.String("from", "", "primera hora evaluada, RFC3339 UTC (obligatorio)")
	toRaw := flag.String("to", "", "última hora evaluada, RFC3339 UTC (obligatorio)")
	mode := flag.String("mode", bootstrap.ReplayModeAsOf, "asof | fixed")
	fixedAsOfRaw := flag.String("fixed-asof", "", "solo -mode fixed: corte de la línea base, RFC3339 (default: -to)")
	outPath := flag.String("out", "", "archivo CSV de salida (default: replay_<point>_<mode>.csv)")
	flag.Parse()

	from, errFrom := parseTime(*fromRaw)
	to, errTo := parseTime(*toRaw)
	if *pointID <= 0 || errFrom != nil || errTo != nil {
		log.Error("-point, -from y -to son obligatorios (-from/-to en RFC3339)")
		flag.Usage()
		os.Exit(2)
	}

	var fixedAsOf time.Time
	if *fixedAsOfRaw != "" {
		parsed, err := parseTime(*fixedAsOfRaw)
		if err != nil {
			log.Error("-fixed-asof inválido (RFC3339)", "value", *fixedAsOfRaw, "error", err)
			os.Exit(2)
		}
		fixedAsOf = parsed
	}

	path := *outPath
	if path == "" {
		path = fmt.Sprintf("replay_%d_%s.csv", *pointID, *mode)
	}

	file, err := os.Create(path)
	if err != nil {
		log.Error("no fue posible crear el CSV", "path", path, "error", err)
		os.Exit(1)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	started := time.Now()
	summary, err := bootstrap.RunReplay(ctx, bootstrap.ReplayOptions{
		PointID:   *pointID,
		From:      from,
		To:        to,
		Mode:      *mode,
		FixedAsOf: fixedAsOf,
		Out:       file,
	})
	if err != nil {
		log.Error("la reproducción falló", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Punto %d — modo %s — [%s, %s]\n", *pointID, *mode, from.Format(time.RFC3339), to.Format(time.RFC3339))
	fmt.Printf("Horas evaluadas: %d   Filas: %d   CALCULATED: %d   Versiones de línea base: %d\n",
		summary.Hours, summary.Rows, summary.Calculated, summary.BaselineVersions)
	fmt.Printf("CSV: %s   (%s)\n", path, time.Since(started).Round(time.Millisecond))
}

func parseTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
