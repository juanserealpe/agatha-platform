package bootstrap

import (
	"errors"
	"testing"
)

func TestParseModules(t *testing.T) {
	t.Run("vacío usa los módulos por defecto", func(t *testing.T) {
		set, err := ParseModules("  ")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got, want := set.String(), DefaultModules; got != want {
			t.Fatalf("set = %q, se esperaba %q", got, want)
		}
	})

	t.Run("normaliza espacios, mayúsculas y repetidos", func(t *testing.T) {
		set, err := ParseModules(" Dashboard, callback ,dashboard,, ")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got, want := set.String(), "callback,dashboard"; got != want {
			t.Fatalf("set = %q, se esperaba %q", got, want)
		}
		if !set.Has(ModuleCallback) || !set.Has(ModuleDashboard) || set.Has(ModuleBaseline) {
			t.Fatalf("Has no refleja el set %q", set)
		}
	})

	t.Run("String sigue el orden de arranque", func(t *testing.T) {
		set, err := ParseModules("dashboard,callback,baseline")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got, want := set.String(), "baseline,callback,dashboard"; got != want {
			t.Fatalf("set = %q, se esperaba %q", got, want)
		}
	})

	t.Run("rule_engine arranca después de processing", func(t *testing.T) {
		set, err := ParseModules("rule_engine,callback,processing,baseline")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got, want := set.String(), "baseline,processing,rule_engine,callback"; got != want {
			t.Fatalf("set = %q, se esperaba %q", got, want)
		}
		if !set.Has(ModuleRuleEngine) {
			t.Fatal("Has(rule_engine) = false")
		}
	})

	t.Run("alert_management arranca después de rule_engine", func(t *testing.T) {
		set, err := ParseModules("alert_management,rule_engine,processing,baseline")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got, want := set.String(), "baseline,processing,rule_engine,alert_management"; got != want {
			t.Fatalf("set = %q, se esperaba %q", got, want)
		}
	})

	errorCases := []struct {
		name string
		raw  string
	}{
		{"módulo desconocido", "callback,reports"},
		{"callback y dataos a la vez", "callback,dataos,dashboard"},
		{"rule_engine sin processing", "baseline,rule_engine"},
		{"alert_management sin rule_engine", "baseline,processing,alert_management"},
		{"solo separadores", ",,"},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseModules(tt.raw)
			if !errors.Is(err, ErrInvalidModules) {
				t.Fatalf("err = %v, se esperaba ErrInvalidModules", err)
			}
		})
	}
}

func TestModuleSetHasHTTP(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"callback", true},
		{"dashboard", true},
		{"baseline", false},
		{"dataos,baseline", false},
		// rule_engine y alert_management exponen /api/rule-engine y /api/alerts.
		{"baseline,processing,rule_engine", true},
		{"baseline,processing,rule_engine,alert_management", true},
	}

	for _, tt := range tests {
		set, err := ParseModules(tt.raw)
		if err != nil {
			t.Fatalf("%q: error inesperado: %v", tt.raw, err)
		}
		if got := set.HasHTTP(); got != tt.want {
			t.Fatalf("%q: HasHTTP = %v, se esperaba %v", tt.raw, got, tt.want)
		}
	}
}
