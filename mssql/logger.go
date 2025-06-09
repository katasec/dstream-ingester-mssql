package mssql

import (
	"github.com/hashicorp/go-hclog"
	"github.com/katasec/dstream/sdk/logging"
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
		// Initialize with the bare logger for clean output with no prefixes
		pluginLogger = logging.SetupBareLogger()
	}
	return pluginLogger
}
