package logging

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger wraps zerolog.Logger with pac-specific configuration
type Logger struct {
	logger zerolog.Logger
}

// NewLogger creates a new pac logger with clean, minimal output
func NewLogger() *Logger {
	// Configure console writer with clean format and colors
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "", // No timestamp
		NoColor:    false,
		FormatLevel: func(i interface{}) string {
			level := strings.ToUpper(fmt.Sprintf("%s", i))
			switch level {
			case "DEBUG":
				return fmt.Sprintf("\x1b[36m[DEBUG]\x1b[0m") // Cyan
			case "INFO":
				return fmt.Sprintf("\x1b[32m[INFO]\x1b[0m") // Green
			case "WARN":
				return fmt.Sprintf("\x1b[33m[WARN]\x1b[0m") // Yellow
			case "ERROR":
				return fmt.Sprintf("\x1b[31m[ERROR]\x1b[0m") // Red
			default:
				return fmt.Sprintf("[%s]", level)
			}
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
		FormatFieldName: func(i interface{}) string {
			return fmt.Sprintf("%s:", i)
		},
		FormatFieldValue: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
	}

	// Set up the logger with console output and info level by default
	logger := zerolog.New(output).Level(zerolog.InfoLevel)

	// Set global logger
	log.Logger = logger

	return &Logger{logger: logger}
}

// NewLoggerWithLevel creates a new pac logger with a specific log level
func NewLoggerWithLevel(level zerolog.Level) *Logger {
	logger := NewLogger()
	logger.logger = logger.logger.Level(level)
	log.Logger = logger.logger
	return logger
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
}

// Infof logs an info message with formatting
func (l *Logger) Infof(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s\n", fmt.Sprintf(format, args...))
}

// Success logs a success message with special styling
func (l *Logger) Success(msg string) {
	fmt.Fprintf(os.Stderr, "\x1b[32m✓\x1b[0m %s\n", msg)
}

// Successf logs a success message with formatting
func (l *Logger) Successf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\x1b[32m✓\x1b[0m %s\n", fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	fmt.Fprintf(os.Stderr, "\x1b[33m%s\x1b[0m\n", msg)
}

// Warnf logs a warning message with formatting
func (l *Logger) Warnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\x1b[33m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m\n", msg)
}

// Errorf logs an error message with formatting
func (l *Logger) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	if l.logger.GetLevel() <= zerolog.DebugLevel {
		fmt.Fprintf(os.Stderr, "\x1b[36m%s\x1b[0m\n", msg)
	}
}

// Debugf logs a debug message with formatting
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.logger.GetLevel() <= zerolog.DebugLevel {
		fmt.Fprintf(os.Stderr, "\x1b[36m%s\x1b[0m\n", fmt.Sprintf(format, args...))
	}
}

// Global logger instance
var defaultLogger *Logger

// Init initializes the default logger
func Init() {
	defaultLogger = NewLogger()
}

// InitWithLevel initializes the default logger with a specific level
func InitWithLevel(level zerolog.Level) {
	defaultLogger = NewLoggerWithLevel(level)
}

// Default returns the default logger instance
func Default() *Logger {
	if defaultLogger == nil {
		defaultLogger = NewLogger()
	}
	return defaultLogger
}

// Convenience functions that use the default logger
func Info(msg string) {
	Default().Info(msg)
}

func Infof(format string, args ...interface{}) {
	Default().Infof(format, args...)
}

func Success(msg string) {
	Default().Success(msg)
}

func Successf(format string, args ...interface{}) {
	Default().Successf(format, args...)
}

func Warn(msg string) {
	Default().Warn(msg)
}

func Warnf(format string, args ...interface{}) {
	Default().Warnf(format, args...)
}

func Error(msg string) {
	Default().Error(msg)
}

func Errorf(format string, args ...interface{}) {
	Default().Errorf(format, args...)
}

func Debug(msg string) {
	Default().Debug(msg)
}

func Debugf(format string, args ...interface{}) {
	Default().Debugf(format, args...)
}

// CheckErr checks if an error is not nil and logs it with our custom logger
// If the error is not nil, it logs the error and exits the program
func CheckErr(err error) {
	if err != nil {
		Error(err.Error())
		os.Exit(1)
	}
}
