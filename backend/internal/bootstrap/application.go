package bootstrap

import (
	"context"
	"fmt"
	"net/http"
)

// Runner es un proceso de fondo (scheduler) de un módulo habilitado.
type Runner struct {
	Name  string
	Start func(ctx context.Context)
}

// Application es el monolito armado solo con los módulos de un ModuleSet.
type Application struct {
	Modules ModuleSet
	// startup corre en orden y bloqueando ANTES de los runners (RunStartup).
	startup []Runner
	runners []Runner
	routes  []func(*http.ServeMux)
}

// NewApplication cablea los módulos habilitados en orden de arranque: Baseline,
// Processing, Rule Engine (suscripción + HTTP de umbrales + catch-up horario),
// Alert Management (migración antes de Baseline; suscripción, /api/alerts, catch-up y publicación de avisos),
// Notifications (push_devices, suscripciones por canal, stream SSE y su heartbeat), DataOS (catch-up; respaldo si también está Callback), Callback y Dashboard. Cada constructor migra su propio
// esquema (Baseline antes que Processing, que lee sus tablas).
func NewApplication(platform *Platform, modules ModuleSet) (*Application, error) {
	app := &Application{Modules: modules}

	// Alert Management migra primero: crea alerts, de la que Baseline copia
	// (seed) los intervalos a excluir de la línea base.
	var alerts *AlertManagementModule
	if modules.Has(ModuleAlertManagement) {
		var err error
		if alerts, err = NewAlertManagementModule(platform); err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleAlertManagement, err)
		}
	}

	if modules.Has(ModuleBaseline) {
		m, err := NewBaselineModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleBaseline, err)
		}
		// Con Alert Management, cada foto de alerta alimenta las exclusiones.
		if alerts != nil {
			m.SubscribeAlertChanged()
		}
		app.runners = append(app.runners, Runner{Name: "scheduler de línea base", Start: m.NewScheduler().Start})
	}

	if modules.Has(ModuleProcessing) {
		m, err := NewProcessingModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleProcessing, err)
		}
		app.runners = append(app.runners, Runner{Name: "evaluación horaria de processing", Start: m.NewScheduler().Start})
	}

	// Las suscripciones quedan registradas aquí, antes de que arranque cualquier
	// scheduler (Runners): ningún evento se publica sin su consumidor.
	if modules.Has(ModuleRuleEngine) {
		m, err := NewRuleEngineModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleRuleEngine, err)
		}
		m.SubscribeIndicatorEvaluated()
		app.routes = append(app.routes, m.RegisterRoutes)
		app.runners = append(app.runners, Runner{Name: "catch-up del rule engine", Start: m.NewScheduler().Start})
		app.runners = append(app.runners, Runner{Name: "evaluación horaria de movimiento", Start: m.NewMotionScheduler().Start})
	}

	if alerts != nil {
		alerts.SubscribeLevelEvaluated()
		alerts.SubscribeMotionEvaluated()
		app.routes = append(app.routes, alerts.RegisterRoutes)
		app.runners = append(app.runners, alerts.Runners()...)
	}

	if modules.Has(ModuleNotifications) {
		m, err := NewNotificationsModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleNotifications, err)
		}
		if !modules.Has(ModuleAlertManagement) {
			m.log.Warn("notifications sin alert_management: no recibirá eventos de alertas")
		}
		m.Subscribe()
		app.routes = append(app.routes, m.RegisterRoutes)
		app.runners = append(app.runners, m.Runners()...)
	}

	if modules.Has(ModuleDataOS) {
		m, err := NewIngestionModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleDataOS, err)
		}
		ingestionScheduler, err := m.NewScheduler(modules.Has(ModuleCallback))
		if err != nil {
			return nil, fmt.Errorf("módulo %s: crear scheduler: %w", ModuleDataOS, err)
		}
		// El catch-up inicial va antes que Processing: sus evaluaciones son
		// inmutables y una hora sin datos quedaría NO_CALCULABLE para siempre.
		app.startup = append(app.startup, Runner{Name: "catch-up inicial de DataOS", Start: ingestionScheduler.RunOnce})
		app.runners = append(app.runners, Runner{Name: "catch-up periódico de DataOS", Start: ingestionScheduler.Start})
	}

	if modules.Has(ModuleCallback) {
		m, err := NewCallbackModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleCallback, err)
		}
		app.routes = append(app.routes, m.RegisterRoutes)
	}

	if modules.Has(ModuleDashboard) {
		app.routes = append(app.routes, NewDashboardModule(platform).RegisterRoutes)
	}

	return app, nil
}

// RegisterRoutes registra en el mux las rutas de los módulos HTTP habilitados.
func (a *Application) RegisterRoutes(mux *http.ServeMux) {
	for _, register := range a.routes {
		register(mux)
	}
}

// RunStartup ejecuta, en orden y bloqueando, las tareas que deben terminar
// antes de que arranquen los Runners (hoy: el catch-up inicial de DataOS).
func (a *Application) RunStartup(ctx context.Context) {
	for _, task := range a.startup {
		if ctx.Err() != nil {
			return
		}
		task.Start(ctx)
	}
}

// Runners devuelve los schedulers de los módulos habilitados.
func (a *Application) Runners() []Runner {
	return a.runners
}
