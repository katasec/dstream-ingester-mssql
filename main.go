package main

import (
	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/plugins/serve"
	"github.com/katasec/dstream/sdk/logging"
)

func main() {
	// Create a completely bare logger with no prefixes or timestamps
	// for the cleanest possible output
	logger := logging.SetupBareLogger()

	// Set the logger as a global for the mssql package to use
	mssql.SetLogger(logger)

	// Publish the plugin using the universal handshake
	serve.Serve(&mssql.Plugin{}, logger)
}
