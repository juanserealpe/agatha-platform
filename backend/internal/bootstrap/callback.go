package bootstrap

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	ingestionHTTP "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/http"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// callbackTokenEnv es la variable con el token Bearer que envía Dataflow Labs.
const callbackTokenEnv = "AGATHA_CALLBACK_TOKEN"

// CallbackModule expone POST /ingestion/callback: la ingesta principal, que
// recibe los registros que Dataflow Labs empuja. No requiere credenciales de DataOS.
type CallbackModule struct {
	handler *ingestionHTTP.CallbackHandler
}

// NewCallbackModule migra el esquema de Ingestion y cablea el callback sobre su
// persistencia (devices + ingestion_logs + raw_measurements). Si AGATHA_CALLBACK_TOKEN no está
// configurada, arranca igual pero rechaza todas las peticiones con 401.
func NewCallbackModule(platform *Platform) (*CallbackModule, error) {
	logDB := logger.New("DataBaseInit", "Info")
	logCallback := logger.New("IngestionCallback", "Info")

	if err := migrateIngestionSchema(platform.DB, logDB); err != nil {
		return nil, err
	}

	location, err := time.LoadLocation("America/Bogota")
	if err != nil {
		return nil, fmt.Errorf("cargar ubicación: %w", err)
	}

	token := os.Getenv(callbackTokenEnv)
	if token == "" {
		logCallback.Error("la variable del token del callback no está configurada; se rechazarán todas las peticiones con 401",
			"required_variable", callbackTokenEnv)
	}

	service := services.NewCallbackService(
		repositories.NewDeviceRepository(platform.DB),
		repositories.NewUnitOfWork(platform.DB, logCallback),
		services.NewCallbackValidator(location),
		services.NewCallbackMapper(),
		logCallback,
	)

	return &CallbackModule{
		handler: ingestionHTTP.NewCallbackHandler(service, token, logCallback),
	}, nil
}

// RegisterRoutes registra el callback en el mux. Se registra sin método para que
// el handler responda 405 en JSON a todo lo que no sea POST.
func (m *CallbackModule) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(ingestionHTTP.CallbackPath, m.handler.ReceiveRecord)
}
