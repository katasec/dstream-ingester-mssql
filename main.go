package main

import (
	hplugin "github.com/hashicorp/go-plugin"

	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/logging"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {
	// Get dstream logger
	stdLogger := logging.GetLogger()

	// Wrap dstream logger to hclog adapter
	hclogAdapter := logging.NewHcLogAdapter(stdLogger)

	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: serve.Handshake,
		Plugins: map[string]hplugin.Plugin{
			"default": &serve.GenericPlugin{
				Impl: &mssql.Plugin{},
			},
		},
		GRPCServer: hplugin.DefaultGRPCServer,
		Logger:     hclogAdapter,
	})
}
