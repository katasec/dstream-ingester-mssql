package mssql

// IngesterConfig holds the strongly-typed configuration for the MSSQL ingester
type IngesterConfig struct {
	DBConnectionString string
	Tables            []string
	Lock              LockConfig
	IngestQueue       IngestQueueConfig
	Polling           PollingConfig
}

// LockConfig holds configuration for distributed locking
type LockConfig struct {
	Provider         string
	Type             string
	ConnectionString string
	ContainerName    string
}

// IngestQueueConfig holds configuration for the ingest queue
type IngestQueueConfig struct {
	Provider         string
	Type             string
	Name             string
	ConnectionString string
}

// PollingConfig holds configuration for CDC polling
type PollingConfig struct {
	Interval    string
	MaxInterval string
}
