package mssql

import (
	"github.com/hashicorp/go-hclog"
	"github.com/katasec/dstream/pkg/logging"
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
		// Initialize with the SDK plugin logger
		pluginLogger = logging.GetPluginLogger("mssql-plugin")
	}
	return pluginLogger
}
