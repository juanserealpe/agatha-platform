package bootstrap

import (
	"errors"
	"fmt"
	"strings"
)

// ModulesEnv es la variable que elige qué módulos levanta cmd/platform.
const ModulesEnv = "AGATHA_MODULES"

// Module identifica un módulo habilitable del monolito.
type Module string

// Módulos disponibles. El orden de moduleOrder es el orden de arranque.
const (
	ModuleBaseline   Module = "baseline"    // scheduler de recálculo de líneas base
	ModuleProcessing Module = "processing"  // evaluación horaria del índice I; requiere baseline
	ModuleRuleEngine Module = "rule_engine" // consume processing.indicator_evaluated; requiere processing
	ModuleDataOS     Module = "dataos"      // ingesta de contingencia: consulta programada a DataOS
	ModuleCallback   Module = "callback"    // ingesta principal: POST /ingestion/callback
	ModuleDashboard  Module = "dashboard"   // API HTTP de solo lectura: GET /api/dashboard/*
)

var moduleOrder = []Module{ModuleBaseline, ModuleProcessing, ModuleRuleEngine, ModuleDataOS, ModuleCallback, ModuleDashboard}

// DefaultModules se usa cuando AGATHA_MODULES está vacía. processing no entra:
// se habilita a mano, después de cargar el histórico de Baseline.
const DefaultModules = "baseline,callback,dashboard"

// ErrInvalidModules indica una selección de módulos desconocida o inconsistente.
var ErrInvalidModules = errors.New("selección de módulos inválida")

// ModuleSet es una selección de módulos ya validada.
type ModuleSet map[Module]bool

// ParseModules interpreta una lista separada por comas (sin distinguir
// mayúsculas, ignorando espacios y repetidos) y valida sus dependencias:
//   - processing requiere baseline (lee sus tablas, que migra Baseline);
//   - rule_engine requiere processing (sin él no le llega ningún evento);
//   - callback y dataos son excluyentes (duplicarían raw_measurements).
//
// Una lista vacía equivale a DefaultModules.
func ParseModules(raw string) (ModuleSet, error) {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultModules
	}

	set := ModuleSet{}
	for _, part := range strings.Split(raw, ",") {
		name := Module(strings.ToLower(strings.TrimSpace(part)))
		if name == "" {
			continue
		}
		if !isKnownModule(name) {
			return nil, fmt.Errorf("%w: módulo desconocido %q (disponibles: %s)", ErrInvalidModules, name, joinModules(moduleOrder))
		}
		set[name] = true
	}

	if len(set) == 0 {
		return nil, fmt.Errorf("%w: no se habilitó ningún módulo", ErrInvalidModules)
	}
	if set.Has(ModuleProcessing) && !set.Has(ModuleBaseline) {
		return nil, fmt.Errorf("%w: %s requiere %s", ErrInvalidModules, ModuleProcessing, ModuleBaseline)
	}
	if set.Has(ModuleRuleEngine) && !set.Has(ModuleProcessing) {
		return nil, fmt.Errorf("%w: %s requiere %s", ErrInvalidModules, ModuleRuleEngine, ModuleProcessing)
	}
	if set.Has(ModuleCallback) && set.Has(ModuleDataOS) {
		return nil, fmt.Errorf("%w: %s y %s no pueden correr a la vez (duplicarían raw_measurements)",
			ErrInvalidModules, ModuleCallback, ModuleDataOS)
	}

	return set, nil
}

// Has indica si el módulo está habilitado.
func (s ModuleSet) Has(m Module) bool { return s[m] }

// HasHTTP indica si algún módulo habilitado expone rutas HTTP.
func (s ModuleSet) HasHTTP() bool { return s.Has(ModuleCallback) || s.Has(ModuleDashboard) }

// String devuelve los módulos habilitados en orden de arranque.
func (s ModuleSet) String() string {
	enabled := make([]Module, 0, len(s))
	for _, m := range moduleOrder {
		if s.Has(m) {
			enabled = append(enabled, m)
		}
	}
	return joinModules(enabled)
}

func isKnownModule(name Module) bool {
	for _, m := range moduleOrder {
		if m == name {
			return true
		}
	}
	return false
}

func joinModules(modules []Module) string {
	names := make([]string, len(modules))
	for i, m := range modules {
		names[i] = string(m)
	}
	return strings.Join(names, ",")
}
