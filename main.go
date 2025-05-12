package main

import (
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {
	fmt.Println("🔌 Starting MSSQL plugin...")
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: serve.Handshake,
		Plugins: map[string]plugin.Plugin{
			"ingester": &serve.IngesterPlugin{
				Impl: mssql.New(),
			},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
