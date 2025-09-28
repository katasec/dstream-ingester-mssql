package types

import "context"

// ChangeType represents the type of change detected in a table
type ChangeType string

const (
	// Insert represents a new row being added
	Insert ChangeType = "insert"
	// Update represents a row being modified
	Update ChangeType = "update"
	// Delete represents a row being removed
	Delete ChangeType = "delete"
)

// ChangeEvent represents a change detected in a table
type ChangeEvent struct {
	TableName  string                 `json:"table_name"`
	ChangeType ChangeType             `json:"change_type"`
	Data       map[string]interface{} `json:"data"`
	Timestamp  string                 `json:"timestamp"`
	LSN        string                 `json:"lsn"`
}

// OutputEnvelope represents the JSON envelope format for stdout output
// This includes the change data plus metadata about the table
type OutputEnvelope struct {
	TableName string                 `json:"table_name"`
	ServerName string                `json:"server_name"`
	Changes   []ChangeEvent          `json:"changes"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TableMonitor defines the interface for monitoring database table changes
type TableMonitor interface {
	// MonitorTable starts monitoring the specified table for changes
	MonitorTable(ctx context.Context) error

	// GetTableName returns the name of the table being monitored
	GetTableName() string
}

// TableMonitorFactory creates table monitors for specific tables
type TableMonitorFactory interface {
	// CreateMonitor creates a new table monitor for the specified table
	CreateMonitor(tableName string) (TableMonitor, error)
}