package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/bootstrap"
	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
)

// Configuración de la carga histórica.
const (
	measuringPointID int64 = 11556
	// Supabase falla con rangos mayores a 5 días: cada consulta a DataOS se
	// limita a esta ventana.
	maxPartition = 5 * 24 * time.Hour
)

// cmd/ingestion ejecuta SOLO el módulo de Ingestion (Platform + Ingestion),
// sin Baseline ni Processing, para cargas históricas largas. Los eventos
// MeasurementsIngested se publican en un bus sin suscriptores y se descartan.
func main() {
	platform, err := bootstrap.NewPlatform()
	if err != nil {
		os.Exit(1)
	}

	ingestion, err := bootstrap.NewIngestionModule(platform)
	if err != nil {
		os.Exit(1)
	}

	log := ingestion.Logger

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := platform.Close(shutdownCtx); err != nil {
			log.Error("el bus de eventos no terminó de drenar antes del cierre", "error", err)
		}
	}()

	location, err := time.LoadLocation("America/Bogota")
	if err != nil {
		log.Error("no fue posible cargar la zona horaria de Colombia", "error", err)
		os.Exit(1)
	}

	start := time.Date(2026, 10, 8, 23, 10, 0, 0, location)
	end := time.Now().In(location) // o una fecha fija: time.Date(2026, 10, 6, 9, 55, 0, 0, location)

	ctx := context.Background()
	partitions := 0

	// IngestDataOs consulta DataOS desde start+1s, así que cada partición
	// arranca exactamente donde terminó la anterior: sin huecos ni solapes.
	for from := start; from.Before(end); {
		to := from.Add(maxPartition)
		if to.After(end) {
			to = end
		}
		partitions++

		log.Info("ingiriendo partición", "partition", partitions, "from", from, "to", to)

		err := ingestion.IngestService.IngestDataOs(ctx, measuringPointID, from, to)
		switch {
		case errors.Is(err, ingestionErrors.ErrRecivingIngestedData):
			// Partición sin datos (equipo apagado, etc.): no aborta la carga.
			log.Warn("partición sin mediciones; se continúa", "partition", partitions, "from", from, "to", to)
		case err != nil:
			// Se aborta para poder reanudar manualmente desde esta partición.
			log.Error("falló la partición; reanudar desde 'from'", "partition", partitions, "from", from, "to", to, "error", err)
			return
		}

		from = to
	}

	log.Info("carga histórica finalizada", "partitions", partitions, "start", start, "end", end)
}
