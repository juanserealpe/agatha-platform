package logger

import (
	"log"
	"strings"
	"time"

	// Embebe la base de zonas horarias en el binario: America/Bogota se resuelve
	// aunque el host (VPS, contenedor) no tenga tzdata instalada.
	_ "time/tzdata"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

const (
	timestampLayout = "2006-01-02 15:04:05 -07:00"
	fieldTimeLayout = time.RFC3339
)

// bogota es la zona de todos los logs, independiente de la zona del host.
var bogota = loadBogota()

type Logger struct {
	module string
	level  Level
}

func loadBogota() *time.Location {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		// Con time/tzdata embebida no debería ocurrir; Colombia no tiene horario de verano.
		return time.FixedZone("America/Bogota", -5*60*60)
	}
	return loc
}

// Configurar zona horaria de Colombia (Bogotá) para todo el proceso y dejar un
// único timestamp por línea (el propio, con offset).
func init() {
	time.Local = bogota
	log.SetFlags(0)
	log.Printf("[%s] Timezone configurado a: America/Bogota (UTC-5)", time.Now().In(bogota).Format(timestampLayout))
}

func New(module string, level string) *Logger {
	return &Logger{
		module: module,
		level:  Level(strings.ToUpper(level)),
	}
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.log(LevelInfo, msg, keysAndValues...)
}

func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(LevelWarn, msg, keysAndValues...)
}

func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.log(LevelError, msg, keysAndValues...)
}

func (l *Logger) log(
	level Level,
	msg string,
	keysAndValues ...interface{},
) {
	timestamp := time.Now().In(bogota).Format(timestampLayout)

	log.Printf(
		"[%s] [%s] [%s] %s %v",
		timestamp,
		level,
		l.module,
		msg,
		normalizeTimes(keysAndValues),
	)
}

// normalizeTimes convierte los campos time.Time a hora Bogotá, sin importar la
// zona con la que llegaron (UTC de la BD, offset del payload, etc.).
func normalizeTimes(keysAndValues []interface{}) []interface{} {
	normalized := make([]interface{}, len(keysAndValues))
	for i, value := range keysAndValues {
		switch v := value.(type) {
		case time.Time:
			normalized[i] = v.In(bogota).Format(fieldTimeLayout)
		case *time.Time:
			if v != nil {
				normalized[i] = v.In(bogota).Format(fieldTimeLayout)
			} else {
				normalized[i] = v
			}
		default:
			normalized[i] = value
		}
	}
	return normalized
}
