package main

import (
	hplugin "github.com/hashicorp/go-plugin"

	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {

	// 🚀 Start plugin with handshake and your logger
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: serve.Handshake,
		Plugins: map[string]hplugin.Plugin{
			"default": &serve.GenericPlugin{
				Impl: &mssql.Plugin{},
			},
		},
		GRPCServer: hplugin.DefaultGRPCServer,
	})
}
