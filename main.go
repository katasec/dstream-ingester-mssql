package main

import (
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"

	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/logging"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

func main() {
	// 🔒 Ensure stdlib log doesn't interfere with go-plugin handshake
	log.SetOutput(os.Stderr)

	// 🎯 Wrap your custom logger with hclog-compatible adapter
	stdLogger := logging.GetLogger()
	hclogAdapter := logging.NewHcLogAdapter(stdLogger)
	hclogAdapter.SetLevel(hclog.Warn)

	// 🚀 Start plugin with handshake and your logger
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: serve.Handshake,
		Plugins: map[string]hplugin.Plugin{
			"default": &serve.GenericPlugin{
				Impl: &mssql.Plugin{},
			},
		},
		GRPCServer: hplugin.DefaultGRPCServer,
		Logger:     hclogAdapter, // ✅ Now safely integrated
	})
}
