package bootstrap

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// DatabaseTargetEnv elige contra qué BD corre un comando desde la PC:
//
//	AGATHA_DB=local       → .env.local      (Postgres en Docker, ver docker-compose.local.yml)
//	AGATHA_DB=production  → .env.production (Supabase del piloto)
//
// El archivo elegido se carga primero y PISA lo que ya haya en el entorno
// (la elección explícita gana sobre un DATABASE_URL exportado); después .env
// completa lo común (DataOS, FCM…) sin pisar nada. Sin AGATHA_DB se carga
// solo .env, como siempre: el contenedor del VPS no cambia.
const DatabaseTargetEnv = "AGATHA_DB"

// DatabaseTarget es la BD elegida con AGATHA_DB.
type DatabaseTarget string

const (
	// DatabaseTargetUnset: AGATHA_DB no está definida (comportamiento histórico).
	DatabaseTargetUnset      DatabaseTarget = ""
	DatabaseTargetLocal      DatabaseTarget = "local"
	DatabaseTargetProduction DatabaseTarget = "production"
)

// ErrInvalidDatabaseTarget indica un AGATHA_DB desconocido o un archivo de
// destino ausente o inconsistente.
var ErrInvalidDatabaseTarget = errors.New("AGATHA_DB inválida")

// LoadEnvironment carga las variables de entorno desde dir según AGATHA_DB.
func LoadEnvironment(dir string) (DatabaseTarget, error) {
	target := DatabaseTarget(strings.ToLower(strings.TrimSpace(os.Getenv(DatabaseTargetEnv))))

	switch target {
	case DatabaseTargetUnset:
		// Sin .env se usan las variables del proceso (contenedor).
		_ = godotenv.Load(filepath.Join(dir, ".env"))
		return target, nil
	case DatabaseTargetLocal, DatabaseTargetProduction:
	default:
		return "", fmt.Errorf("%w: %q (valores: %s, %s)", ErrInvalidDatabaseTarget, target, DatabaseTargetLocal, DatabaseTargetProduction)
	}

	file := filepath.Join(dir, ".env."+string(target))
	vars, err := godotenv.Read(file)
	if err != nil {
		return "", fmt.Errorf("%w: no se pudo leer %s: %v", ErrInvalidDatabaseTarget, file, err)
	}

	dsn := strings.TrimSpace(vars["DATABASE_URL"])
	if dsn == "" {
		return "", fmt.Errorf("%w: %s no define DATABASE_URL", ErrInvalidDatabaseTarget, file)
	}
	if target == DatabaseTargetLocal && !isLoopbackDSN(dsn) {
		return "", fmt.Errorf("%w: %s apunta a %s, que no es local", ErrInvalidDatabaseTarget, file, DescribeDatabaseURL(dsn))
	}

	for key, value := range vars {
		if err := os.Setenv(key, value); err != nil {
			return "", fmt.Errorf("definir %s: %w", key, err)
		}
	}

	// .env completa lo común sin pisar lo anterior.
	_ = godotenv.Load(filepath.Join(dir, ".env"))
	return target, nil
}

// DescribeDatabaseURL devuelve host:puerto/base de un DSN en formato URL, sin
// usuario ni contraseña, para poder loguear contra qué BD se corre.
func DescribeDatabaseURL(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host == "" {
		return "(DSN sin formato URL)"
	}
	return parsed.Host + parsed.Path
}

// isLoopbackDSN indica si el DSN (formato URL) apunta a esta máquina.
func isLoopbackDSN(dsn string) bool {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
