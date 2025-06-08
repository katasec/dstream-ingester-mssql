package main

import (
	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/logging"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {
	// Create a plugin logger using the SDK's SetupPluginLogger
	// This creates a logger with no timestamps (host will add these)
	logger := logging.SetupPluginLogger("mssql-plugin")

	// Set the logger as a global for the mssql package to use
	mssql.SetLogger(logger)

	// Publish the plugin using the universal handshake
	serve.Serve(&mssql.Plugin{}, logger)
}
