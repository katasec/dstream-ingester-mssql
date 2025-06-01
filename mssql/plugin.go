package mssql

import (
	"context"
	"fmt"

	"github.com/katasec/dstream/pkg/logging"
	pb "github.com/katasec/dstream/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// Plugin implements the universal plugins.Plugin interface.
type Plugin struct{}

// ───────────────────────────────────────────────────────────────────────────────
//
//	Start receives the entire HCL `config { … }` block as google.protobuf.Struct
//
// ───────────────────────────────────────────────────────────────────────────────
func (p *Plugin) Start(ctx context.Context, cfg *structpb.Struct) error {
	log := logging.GetLogger()
	raw := cfg.AsMap()

	// --- required: db_connection_string ---------------------------------------------------------
	connStr, ok := raw["db_connection_string"].(string)
	if !ok || connStr == "" {
		return fmt.Errorf("missing required config: db_connection_string")
	}

	// --- required: tables ----------------------------------------------------------------------
	var tables []string
	switch v := raw["tables"].(type) {
	case []interface{}:
		for _, t := range v {
			if s, ok := t.(string); ok {
				tables = append(tables, s)
			}
		}
	case string: // fallback for old flattened config
		if v != "" {
			tables = append(tables, v)
		}
	default:
		return fmt.Errorf("tables must be a list of strings")
	}
	if len(tables) == 0 {
		return fmt.Errorf("missing required config: tables")
	}

	log.Debug("[MSSQLPlugin] ConnStr:", connStr)
	log.Debug("[MSSQLPlugin] Tables :", tables)

	return StartFromConfig(ctx, connStr, tables)
}

// ───────────────────────────────────────────────────────────────────────────────
//
//	Schema advertises hierarchical fields so the CLI can validate / prompt
//
// ───────────────────────────────────────────────────────────────────────────────
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
			Description: "List of tables to monitor for CDC",
		},
		// ── nested blocks (object type) ───────────────────────────────────
		{
			Name:        "ingest_queue",
			Type:        pb.FieldTypeObject,
			Required:    false,
			Description: "Settings for publishing CDC events downstream",
			Fields: []*pb.FieldSchema{
				{Name: "provider", Type: pb.FieldTypeString, Required: true},
				{Name: "type", Type: pb.FieldTypeString, Required: true},
				{Name: "name", Type: pb.FieldTypeString, Required: true},
				{Name: "connection_string", Type: pb.FieldTypeString, Required: true},
			},
		},
		{
			Name:        "lock",
			Type:        pb.FieldTypeObject,
			Required:    false,
			Description: "Distributed-lock configuration",
			Fields: []*pb.FieldSchema{
				{Name: "provider", Type: pb.FieldTypeString, Required: true},
				{Name: "type", Type: pb.FieldTypeString, Required: true},
				{Name: "connection_string", Type: pb.FieldTypeString, Required: true},
				{Name: "container_name", Type: pb.FieldTypeString, Required: true},
			},
		},
		{
			Name:        "polling",
			Type:        pb.FieldTypeObject,
			Required:    false,
			Description: "Poll/back-off tuning",
			Fields: []*pb.FieldSchema{
				{Name: "interval", Type: pb.FieldTypeString, Required: false},
				{Name: "max_interval", Type: pb.FieldTypeString, Required: false},
			},
		},
	}, nil
}
