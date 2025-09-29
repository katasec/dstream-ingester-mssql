package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// Log levels
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError

	// ANSI color codes
	colorReset = "\033[0m"
	colorDebug = "\033[38;5;215m" // Debug - soft orange
	colorInfo  = "\033[38;5;78m"  // Info - brighter leafy green
	colorWarn  = "\033[38;5;220m" // kind of an amber/golden sand
	colorError = "\033[38;5;167m" // Error - toned-down crimson
)

var (
	// Global logger instance
	globalLogger Logger
	once         sync.Once

	// Environment variable names
	envLogLevel = "DSTREAM_LOG_LEVEL" // debug, info, warn, error
)

// Logger interface defines the logging methods
type Logger interface {
	// Standard log package compatible methods
	Printf(format string, v ...any)
	Println(v ...any)

	// Structured logging methods
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// stdLogger implements Logger using the standard log package
type stdLogger struct {
	logLevel LogLevel
	debug    *log.Logger
	info     *log.Logger
	warn     *log.Logger
	error    *log.Logger
}

// Initialize the global logger
func init() {
	SetupLogging()
}

// SetupLogging configures the global logger based on environment variables
func SetupLogging() {
	once.Do(func() {
		// Get log level from environment
		logLevel := getLogLevel()

		// Create loggers for each level with appropriate prefixes and colors
		debugLogger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
		infoLogger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
		warnLogger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
		errorLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)

		// Create logger
		globalLogger = &stdLogger{
			logLevel: logLevel,
			debug:    debugLogger,
			info:     infoLogger,
			warn:     warnLogger,
			error:    errorLogger,
		}
	})
}

// GetLogger returns the global logger instance
func GetLogger() Logger {
	return globalLogger
}

// Helper function to get log level from environment
func getLogLevel() LogLevel {
	level := strings.ToLower(os.Getenv(envLogLevel))
	switch level {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// formatMessage formats the message with any additional arguments
func formatMessage(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}

	// Convert args to key-value pairs
	pairs := make([]string, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			pairs = append(pairs, fmt.Sprintf("%v=%v", args[i], args[i+1]))
		}
	}

	// If there are key-value pairs, append them to the message
	if len(pairs) > 0 {
		return fmt.Sprintf("%s [%s]", msg, strings.Join(pairs, " "))
	}
	return msg
}

// Printf provides compatibility with standard log.Printf
func (l *stdLogger) Printf(format string, v ...any) {
	if l.logLevel <= LevelInfo {
		l.info.Printf("%s[INFO]%s %s", colorInfo, colorReset, fmt.Sprintf(format, v...))
	}
}

// Println provides compatibility with standard log.Println
func (l *stdLogger) Println(v ...any) {
	if l.logLevel <= LevelInfo {
		message := fmt.Sprintln(v...)
		// Remove trailing newline that fmt.Sprintln adds
		message = strings.TrimSuffix(message, "\n")
		l.info.Printf("%s[INFO]%s %s", colorInfo, colorReset, message)
	}
}

// Debug logs a debug message
func (l *stdLogger) Debug(msg string, args ...any) {
	if l.logLevel <= LevelDebug {
		l.debug.Printf("%s[DEBUG]%s %s", colorDebug, colorReset, formatMessage(msg, args...))
	}
}

// Info logs an info message
func (l *stdLogger) Info(msg string, args ...any) {
	if l.logLevel <= LevelInfo {
		l.info.Printf("%s[INFO]%s %s", colorInfo, colorReset, formatMessage(msg, args...))
	}
}

// Warn logs a warning message
func (l *stdLogger) Warn(msg string, args ...any) {
	if l.logLevel <= LevelWarn {
		l.warn.Printf("%s[WARN]%s %s", colorWarn, colorReset, formatMessage(msg, args...))
	}
}

// Error logs an error message
func (l *stdLogger) Error(msg string, args ...any) {
	if l.logLevel <= LevelError {
		l.error.Printf("%s[ERROR]%s %s", colorError, colorReset, formatMessage(msg, args...))
	}
}

// SetLogLevel updates the log level of the global logger at runtime
func SetLogLevel(level string) {
	var lvl LogLevel
	switch strings.ToLower(level) {
	case "debug":
		lvl = LevelDebug
	case "warn":
		lvl = LevelWarn
	case "error":
		lvl = LevelError
	default:
		lvl = LevelInfo
	}

	if logger, ok := globalLogger.(*stdLogger); ok {
		logger.logLevel = lvl
	}
}
