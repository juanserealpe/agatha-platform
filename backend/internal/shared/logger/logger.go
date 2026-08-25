package logger

import (
	"log"
	"strings"
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
	log.Printf(
		"[%s] [%s] %s %v",
		level,
		l.module,
		msg,
		keysAndValues,
	)
}
