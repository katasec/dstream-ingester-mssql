package config

import (
	"encoding/json"
	"strings"
	"time"
)

// ProviderConfig holds the configuration for the SQL Server CDC provider
// This will be read from JSON stdin input
type ProviderConfig struct {
	DBConnectionString string     `json:"db_connection_string"` // SQL Server connection string
	PollInterval       string     `json:"poll_interval"`        // How often to poll for changes (e.g., "5s", "30s")
	MaxPollInterval    string     `json:"max_poll_interval"`    // Maximum backoff interval (e.g., "5m")
	LockConfig         LockConfig `json:"lock_config"`          // Distributed locking configuration
	Tables             []string   `json:"tables"`               // List of table names to monitor
}

// GetPollInterval returns the PollInterval as a time.Duration
func (p *ProviderConfig) GetPollInterval() (time.Duration, error) {
	return time.ParseDuration(p.PollInterval)
}

// GetMaxPollInterval returns the MaxPollInterval as a time.Duration  
func (p *ProviderConfig) GetMaxPollInterval() (time.Duration, error) {
	return time.ParseDuration(p.MaxPollInterval)
}

// LockConfig represents the configuration for distributed locking
type LockConfig struct {
	Type             string `json:"type"`              // Lock provider type (e.g., "azure_blob")
	ConnectionString string `json:"connection_string"` // Connection string for the lock provider
	ContainerName    string `json:"container_name"`    // Name of the container used for lock files
}

// LoadConfigFromJSON loads configuration from JSON input
func LoadConfigFromJSON(jsonData []byte) (*ProviderConfig, error) {
	var config ProviderConfig
	err := json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetServerName extracts the server name from a SQL Server connection string
// This is a simplified version for the standalone provider
func GetServerName(connectionString string) string {
	// Simple extraction - look for server= parameter
	// This is a basic implementation - could be enhanced for more complex scenarios
	parts := strings.Split(connectionString, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "server=") {
			serverPart := part[7:] // Remove "server="
			// Handle server names with ports or instances
			if idx := strings.Index(serverPart, ","); idx != -1 {
				return serverPart[:idx]
			}
			if idx := strings.Index(serverPart, "\\"); idx != -1 {
				return serverPart[:idx]
			}
			return serverPart
		}
	}
	return "unknown"
}