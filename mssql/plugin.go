package mssql

import (
	"context"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/katasec/dstream/pkg/plugins"
)

type PluginConfig struct {
	DBConnectionString string   `hcl:"db_connection_string"`
	Tables             []string `hcl:"tables"`
}

type Plugin struct{}

func (p *Plugin) Start(ctx context.Context, rawConfig []byte) error {
	var cfg PluginConfig

	file, diags := hclsyntax.ParseConfig(rawConfig, "plugin_config.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		return diags
	}

	diags = gohcl.DecodeBody(file.Body, nil, &cfg)
	if diags.HasErrors() {
		return diags
	}

	log.Printf("[MSSQLPlugin] Config: DB = %s, Tables = %v", cfg.DBConnectionString, cfg.Tables)

	ing := New()

	return ing.Start(ctx, func(e plugins.Event) error {
		log.Printf("[EVENT] %v", e)
		return nil
	})
}
