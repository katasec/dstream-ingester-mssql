package mssql

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/katasec/dstream/pkg/plugins"
)

// PluginConfig matches the HCL config block
type PluginConfig struct {
	DBConnectionString string   `hcl:"db_connection_string"`
	Tables             []string `hcl:"tables"`
}

type Plugin struct{}

func (p *Plugin) Start(ctx context.Context, rawConfig []byte) error {
	var cfg PluginConfig

	// Parse HCL config bytes
	file, diags := hclsyntax.ParseConfig(rawConfig, "plugin_config.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		return diags
	}

	// Extract the inner "config" block
	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "config"},
		},
	})
	if diags.HasErrors() {
		return diags
	}
	if len(content.Blocks) == 0 {
		return fmt.Errorf("no 'config' block found in task")
	}

	// Decode into Go struct
	diags = gohcl.DecodeBody(content.Blocks[0].Body, nil, &cfg)
	if diags.HasErrors() {
		return diags
	}

	// Log parsed config
	log.Printf("[MSSQLPlugin] Config: DB = %s, Tables = %v", cfg.DBConnectionString, cfg.Tables)

	// Start the actual CDC ingestion
	ing := New()
	return ing.Start(ctx, func(e plugins.Event) error {
		log.Printf("[EVENT] %v", e)
		return nil
	})
}
