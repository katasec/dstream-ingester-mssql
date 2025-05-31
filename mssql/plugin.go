package mssql

import (
	"context"
	"fmt"
	"log"
	"strings"

	pb "github.com/katasec/dstream/proto"
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

	return StartFromConfig(ctx, connStr, tables)
}

func (p *Plugin) GetSchema(ctx context.Context) ([]*pb.FieldSchema, error) {
	return []*pb.FieldSchema{
		{
			Name:        "db_connection_string",
			Type:        pb.FieldTypeString,
			Required:    true,
			Description: "Connection string to connect to the MSSQL database",
		},
		{
			Name:        "tables",
			Type:        pb.FieldTypeList,
			Required:    true,
			Description: "Comma-separated list of tables to monitor for CDC",
		},
	}, nil
}
