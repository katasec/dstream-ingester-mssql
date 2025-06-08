package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {
	// Create a plugin logger that writes to stderr
	logLevel := hclog.Info
	if level := os.Getenv("DSTREAM_LOG_LEVEL"); level != "" {
		logLevel = hclog.LevelFromString(level)
	}

	// Check if JSON logging is enabled
	jsonFormat := false
	if jsonEnv := os.Getenv("DSTREAM_LOG_JSON"); jsonEnv == "true" || jsonEnv == "1" {
		jsonFormat = true
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:       "mssql-plugin",
		Level:      logLevel,
		Output:     os.Stderr, // Important: use stderr, not stdout for plugin logs
		JSONFormat: jsonFormat,
	})

	// Set the logger as a global for the mssql package to use
	mssql.SetLogger(logger)

	// Publish the plugin using the universal handshake
	serve.Serve(&mssql.Plugin{}, logger)
}
