package main

import (
	"context"
	"log"

	hplugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/katasec/dstream-ingester-mssql/mssql"
	"github.com/katasec/dstream/pkg/plugins"
	"github.com/katasec/dstream/pkg/plugins/serve"
)

// PluginConfig matches the config block in your HCL task definition
type PluginConfig struct {
	DBConnectionString string   `hcl:"db_connection_string"`
	Tables             []string `hcl:"tables"`
}

// MSSQLPluginAdapter wraps your real ingestor for use in the generic plugin model
type MSSQLPluginAdapter struct{}

func (p *MSSQLPluginAdapter) Start(ctx context.Context, rawConfig []byte) error {
	var cfg PluginConfig

	// Parse HCL into usable config
	file, diags := hclsyntax.ParseConfig(rawConfig, "plugin_config.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		return diags
	}

	diags = gohcl.DecodeBody(file.Body, nil, &cfg)
	if diags.HasErrors() {
		return diags
	}

	log.Println("[MSSQLPlugin] Parsed config:")
	log.Printf("- DBConnectionString: %s", cfg.DBConnectionString)
	log.Printf("- Tables: %v", cfg.Tables)

	// Launch your existing ingestor
	ing := mssql.New()

	return ing.Start(ctx, func(e plugins.Event) error {
		log.Printf("[EVENT] %v", e)
		return nil
	})
}

func main() {
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: serve.Handshake,
		Plugins: map[string]hplugin.Plugin{
			"default": &serve.GenericPlugin{
				Impl: &MSSQLPluginAdapter{},
			},
		},
		GRPCServer: hplugin.DefaultGRPCServer,
	})
}
