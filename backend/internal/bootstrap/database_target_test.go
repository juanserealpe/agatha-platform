package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeEnvFile crea un archivo de entorno en dir.
func writeEnvFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("no se pudo escribir %s: %v", name, err)
	}
}

// isolateEnv deja las variables en blanco para el test y las restaura al final.
// Las que los archivos definen se borran (no solo se vacían) porque godotenv.Load
// no pisa una variable que existe, aunque esté vacía.
func isolateEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unsetenv %s: %v", key, err)
		}
	}
}

const (
	localURL      = "postgres://agatha:agatha@127.0.0.1:5433/agatha?sslmode=disable"
	productionURL = "postgresql://user.ref:secreto@aws-0-us-west-2.pooler.supabase.com:5432/postgres"
)

func TestLoadEnvironment(t *testing.T) {
	t.Run("sin AGATHA_DB usa .env como siempre (VPS)", func(t *testing.T) {
		isolateEnv(t, DatabaseTargetEnv, "DATABASE_URL", "AGATHA_TEST_COMMON")
		dir := t.TempDir()
		writeEnvFile(t, dir, ".env", "DATABASE_URL="+productionURL+"\nAGATHA_TEST_COMMON=si\n")

		target, err := LoadEnvironment(dir)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if target != DatabaseTargetUnset {
			t.Fatalf("target = %q, se esperaba vacío", target)
		}
		if got := os.Getenv("DATABASE_URL"); got != productionURL {
			t.Fatalf("DATABASE_URL = %q", got)
		}
	})

	t.Run("sin AGATHA_DB respeta el entorno del proceso (contenedor)", func(t *testing.T) {
		isolateEnv(t, DatabaseTargetEnv)
		t.Setenv("DATABASE_URL", productionURL)

		if _, err := LoadEnvironment(t.TempDir()); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got := os.Getenv("DATABASE_URL"); got != productionURL {
			t.Fatalf("DATABASE_URL = %q", got)
		}
	})

	t.Run("local toma la URL de .env.local y el resto de .env", func(t *testing.T) {
		isolateEnv(t, "DATABASE_URL", "AGATHA_TEST_COMMON")
		t.Setenv(DatabaseTargetEnv, " Local ")
		dir := t.TempDir()
		writeEnvFile(t, dir, ".env", "DATABASE_URL="+productionURL+"\nAGATHA_TEST_COMMON=si\n")
		writeEnvFile(t, dir, ".env.local", "DATABASE_URL="+localURL+"\n")

		target, err := LoadEnvironment(dir)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if target != DatabaseTargetLocal {
			t.Fatalf("target = %q", target)
		}
		if got := os.Getenv("DATABASE_URL"); got != localURL {
			t.Fatalf("DATABASE_URL = %q, se esperaba la local", got)
		}
		if got := os.Getenv("AGATHA_TEST_COMMON"); got != "si" {
			t.Fatalf("las variables comunes de .env no se cargaron: %q", got)
		}
	})

	t.Run("la URL del archivo elegido pisa una DATABASE_URL exportada", func(t *testing.T) {
		t.Setenv(DatabaseTargetEnv, "local")
		t.Setenv("DATABASE_URL", productionURL)
		dir := t.TempDir()
		writeEnvFile(t, dir, ".env.local", "DATABASE_URL="+localURL+"\n")

		if _, err := LoadEnvironment(dir); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got := os.Getenv("DATABASE_URL"); got != localURL {
			t.Fatalf("DATABASE_URL = %q, la elección explícita debe ganar", got)
		}
	})

	t.Run("production carga .env.production con su AGATHA_ENV", func(t *testing.T) {
		isolateEnv(t, "DATABASE_URL", EnvironmentEnv)
		t.Setenv(DatabaseTargetEnv, "production")
		dir := t.TempDir()
		writeEnvFile(t, dir, ".env.production", "DATABASE_URL="+productionURL+"\nAGATHA_ENV=production\n")

		target, err := LoadEnvironment(dir)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if target != DatabaseTargetProduction {
			t.Fatalf("target = %q", target)
		}
		if !isProductionEnvironment() {
			t.Fatal("AGATHA_ENV de .env.production no quedó cargada")
		}
	})

	t.Run("valor desconocido falla", func(t *testing.T) {
		t.Setenv(DatabaseTargetEnv, "staging")
		if _, err := LoadEnvironment(t.TempDir()); !errors.Is(err, ErrInvalidDatabaseTarget) {
			t.Fatalf("err = %v, se esperaba ErrInvalidDatabaseTarget", err)
		}
	})

	t.Run("falta el archivo del destino", func(t *testing.T) {
		t.Setenv(DatabaseTargetEnv, "production")
		if _, err := LoadEnvironment(t.TempDir()); !errors.Is(err, ErrInvalidDatabaseTarget) {
			t.Fatalf("err = %v, se esperaba ErrInvalidDatabaseTarget", err)
		}
	})

	t.Run("el archivo del destino no define DATABASE_URL", func(t *testing.T) {
		t.Setenv(DatabaseTargetEnv, "local")
		dir := t.TempDir()
		writeEnvFile(t, dir, ".env.local", "OTRA=1\n")
		if _, err := LoadEnvironment(dir); !errors.Is(err, ErrInvalidDatabaseTarget) {
			t.Fatalf("err = %v, se esperaba ErrInvalidDatabaseTarget", err)
		}
	})

	t.Run("local que apunta a un host remoto falla", func(t *testing.T) {
		t.Setenv(DatabaseTargetEnv, "local")
		dir := t.TempDir()
		writeEnvFile(t, dir, ".env.local", "DATABASE_URL="+productionURL+"\n")
		if _, err := LoadEnvironment(dir); !errors.Is(err, ErrInvalidDatabaseTarget) {
			t.Fatalf("err = %v, se esperaba ErrInvalidDatabaseTarget", err)
		}
	})
}

func TestDescribeDatabaseURL(t *testing.T) {
	cases := []struct {
		dsn  string
		want string
	}{
		{localURL, "127.0.0.1:5433/agatha"},
		{productionURL, "aws-0-us-west-2.pooler.supabase.com:5432/postgres"},
		{"host=localhost dbname=x password=secreto", "(DSN sin formato URL)"},
	}
	for _, c := range cases {
		if got := DescribeDatabaseURL(c.dsn); got != c.want {
			t.Errorf("DescribeDatabaseURL(%q) = %q, se esperaba %q", c.dsn, got, c.want)
		}
	}
}
