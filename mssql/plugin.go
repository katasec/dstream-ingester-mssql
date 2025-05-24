package mssql

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type Plugin struct{}

func (p *Plugin) Start(ctx context.Context, cfg map[string]string) error {
	log.Println("[MSSQLPlugin] Received config:", cfg)

	connStr := cfg["db_connection_string"]
	if connStr == "" {
		return fmt.Errorf("missing required config: db_connection_string")
	}

	rawTables := cfg["tables"]
	if rawTables == "" {
		return fmt.Errorf("missing required config: tables")
	}

	// Parse tables as comma-separated string
	tables := strings.Split(rawTables, ",")

	log.Printf("Connecting to DB: %s", connStr)
	log.Printf("Monitoring tables: %v", tables)

	// Start ingestion with parsed config
	return StartFromConfig(ctx, connStr, tables)
}
