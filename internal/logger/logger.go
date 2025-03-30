package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Level represents a log level
type Level int

const (
	// Debug level logs detailed information for debugging
	Debug Level = iota
	// Info level logs general information about normal operation
	Info
	// Warn level logs warning messages
	Warn
	// Error level logs error messages
	Error
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a log level from a string
func ParseLevel(level string) Level {
	switch strings.ToLower(level) {
	case "debug":
		return Debug
	case "info":
		return Info
	case "warn", "warning":
		return Warn
	case "error", "err":
		return Error
	default:
		return Info
	}
}

// Logger is a simple logger with support for log levels
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new logger with the specified level
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

// NewFromString creates a new logger with the level parsed from a string
func NewFromString(level string) *Logger {
	return New(ParseLevel(level))
}

// SetLevel changes the logger's level
func (l *Logger) SetLevel(level Level) {
	l.level = level
}

// log writes a log message with the specified level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("[%s] %s: %s", timestamp, level.String(), message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(Debug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(Info, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(Warn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(Error, format, args...)
}