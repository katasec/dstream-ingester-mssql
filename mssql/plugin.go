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

	// Validate and convert config to IngesterConfig
	ingesterConfig, err := validateConfig(cfg)
	if err != nil {
		return err
	}

	log.Debug("[MSSQLPlugin] ConnStr:", ingesterConfig.DBConnectionString)
	log.Debug("[MSSQLPlugin] Tables :", ingesterConfig.Tables)

	return StartFromConfig(ctx, ingesterConfig)
}

// validateConfig validates the plugin configuration and returns a strongly-typed IngesterConfig
func validateConfig(cfg *structpb.Struct) (*IngesterConfig, error) {
	log := logging.GetLogger()
	raw := cfg.AsMap()
	log.Debug("Struct Config map:", cfg)

	// Initialize the config struct
	config := &IngesterConfig{}

	// --- required: db_connection_string ---------------------------------------------------------
	connStr, ok := raw["db_connection_string"].(string)
	log.Debug("Raw Config map:", raw)
	if !ok || connStr == "" {
		return nil, fmt.Errorf("missing required config: db_connection_string")
	}
	config.DBConnectionString = connStr

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
		return nil, fmt.Errorf("tables must be a list of strings")
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("missing required config: tables")
	}
	config.Tables = tables

	// --- required: lock configuration ---------------------------------------------------------
	lockConfig, ok := raw["lock"].(map[string]interface{})
	if !ok || lockConfig == nil {
		return nil, fmt.Errorf("missing required config: lock")
	}

	// Validate lock type
	lockType, ok := lockConfig["type"].(string)
	if !ok || lockType == "" {
		return nil, fmt.Errorf("missing required config: lock.type")
	}
	config.Lock.Type = lockType

	// Validate lock provider
	lockProvider, ok := lockConfig["provider"].(string)
	if !ok || lockProvider == "" {
		return nil, fmt.Errorf("missing required config: lock.provider")
	}
	config.Lock.Provider = lockProvider

	// Validate lock connection string
	lockConnStr, ok := lockConfig["connection_string"].(string)
	if !ok || lockConnStr == "" {
		return nil, fmt.Errorf("missing required config: lock.connection_string")
	}
	config.Lock.ConnectionString = lockConnStr

	// Validate lock container name
	lockContainer, ok := lockConfig["container_name"].(string)
	if !ok || lockContainer == "" {
		return nil, fmt.Errorf("missing required config: lock.container_name")
	}
	config.Lock.ContainerName = lockContainer

	// --- optional: ingest_queue configuration -------------------------------------------------
	if ingestQueue, ok := raw["ingest_queue"].(map[string]interface{}); ok && ingestQueue != nil {
		if provider, ok := ingestQueue["provider"].(string); ok {
			config.IngestQueue.Provider = provider
		}
		if queueType, ok := ingestQueue["type"].(string); ok {
			config.IngestQueue.Type = queueType
		}
		if name, ok := ingestQueue["name"].(string); ok {
			config.IngestQueue.Name = name
		}
		if connStr, ok := ingestQueue["connection_string"].(string); ok {
			config.IngestQueue.ConnectionString = connStr
		}
	}

	// --- optional: polling configuration -----------------------------------------------------
	if polling, ok := raw["polling"].(map[string]interface{}); ok && polling != nil {
		if interval, ok := polling["interval"].(string); ok {
			config.Polling.Interval = interval
		}
		if maxInterval, ok := polling["max_interval"].(string); ok {
			config.Polling.MaxInterval = maxInterval
		}
	}

	return config, nil
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
			Required:    true,
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
