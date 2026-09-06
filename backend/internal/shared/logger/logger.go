package logger

import (
	"log"
	"strings"
	"time"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Logger struct {
	module string
	level  Level
}

// Configurar zona horaria de Colombia (Bogotá)
func init() {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		log.Printf("Error loading timezone America/Bogota: %v, usando UTC", err)
	} else {
		time.Local = loc
		log.Printf("Timezone configurado a: America/Bogota (UTC-5)")
	}
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
	// Obtener hora actual en zona horaria de Bogotá
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	log.Printf(
		"[%s] [%s] [%s] %s %v",
		timestamp,
		level,
		l.module,
		msg,
		keysAndValues,
	)
}
