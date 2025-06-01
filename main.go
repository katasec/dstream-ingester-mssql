package main

import (
	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {
	// One-liner publishes the plugin using the new universal handshake.
	serve.Serve(&mssql.Plugin{})

}
