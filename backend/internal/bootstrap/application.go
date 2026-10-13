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
	runners []Runner
	routes  []func(*http.ServeMux)
}

// NewApplication cablea los módulos habilitados en orden de arranque: Baseline,
// DataOS, Callback y Dashboard. Cada constructor migra su propio esquema.
func NewApplication(platform *Platform, modules ModuleSet) (*Application, error) {
	app := &Application{Modules: modules}

	if modules.Has(ModuleBaseline) {
		m, err := NewBaselineModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleBaseline, err)
		}
		app.runners = append(app.runners, Runner{Name: "scheduler de línea base", Start: m.NewScheduler().Start})
	}

	if modules.Has(ModuleDataOS) {
		m, err := NewIngestionModule(platform)
		if err != nil {
			return nil, fmt.Errorf("módulo %s: %w", ModuleDataOS, err)
		}
		ingestionScheduler, err := m.NewScheduler()
		if err != nil {
			return nil, fmt.Errorf("módulo %s: crear scheduler: %w", ModuleDataOS, err)
		}
		app.runners = append(app.runners, Runner{Name: "scheduler de ingesta DataOS", Start: ingestionScheduler.Start})
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

// Runners devuelve los schedulers de los módulos habilitados.
func (a *Application) Runners() []Runner {
	return a.runners
}
