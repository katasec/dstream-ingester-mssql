package mssql

import (
	"github.com/hashicorp/go-hclog"
)

// Default logger
var pluginLogger hclog.Logger

// SetLogger sets the global logger for the mssql package
func SetLogger(logger hclog.Logger) {
	pluginLogger = logger
}

// GetLogger returns the global logger for the mssql package
func GetLogger() hclog.Logger {
	if pluginLogger == nil {
		// Return a null logger if not initialized
		return hclog.NewNullLogger()
	}
	return pluginLogger
}
