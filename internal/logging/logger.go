package logging

import "log"

// Logger provides the same interface as the original logging package
type Logger struct{}

// GetLogger returns a logger instance (matches original interface)
func GetLogger() *Logger {
	return &Logger{}
}

// Printf matches the original logger interface
func (l *Logger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Error matches the original logger interface
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	log.Printf("ERROR: %s %v", msg, keysAndValues)
}

// Info matches the original logger interface
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	log.Printf("INFO: %s %v", msg, keysAndValues)
}

// Debug matches the original logger interface
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	log.Printf("DEBUG: %s %v", msg, keysAndValues)
}