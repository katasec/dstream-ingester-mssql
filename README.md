# DStream SQL Server CDC Provider

A modern, standalone SQL Server Change Data Capture (CDC) provider for the DStream ecosystem. This provider monitors SQL Server tables for changes and outputs structured JSON events via stdout, following the modern DStream provider architecture pattern.

## Overview

This provider extracts CDC changes from SQL Server tables and outputs them as JSON envelopes. It supports concurrent monitoring of multiple tables with distributed locking, exponential backoff, and checkpoint management for reliable change tracking.

## Architecture

### Input/Output Model
- **Input**: JSON configuration via stdin
- **Processing**: Concurrent table monitoring with distributed coordination
- **Output**: JSON envelopes with change events via stdout

### Key Features
- 🔄 **Concurrent Multi-Table Monitoring**: Each table runs in its own goroutine
- 🔐 **Distributed Locking**: Prevents duplicate processing across multiple instances
- 📊 **Checkpoint Management**: Tracks LSN (Log Sequence Number) progress per table
- ⚡ **Exponential Backoff**: Intelligent polling interval adjustment
- 🛡️ **Error Handling**: Graceful shutdown and error recovery
- 📋 **Simplified Configuration**: Shared settings with table list

## Configuration

### JSON Configuration Format
```json
{
  "db_connection_string": "server=localhost;database=testdb;trusted_connection=true",
  "poll_interval": "5s",
  "max_poll_interval": "5m", 
  "lock_config": {
    "type": "azure_blob",
    "connection_string": "DefaultEndpointsProtocol=https;AccountName=storage;AccountKey=key==;EndpointSuffix=core.windows.net",
    "container_name": "locks"
  },
  "tables": [
    "dbo.customers",
    "dbo.orders",
    "dbo.products"
  ]
}
```

### Configuration Parameters
- **`db_connection_string`**: SQL Server connection string
- **`poll_interval`**: Base polling interval for CDC changes (e.g., "5s", "30s")
- **`max_poll_interval`**: Maximum backoff interval (e.g., "5m", "1h") 
- **`lock_config`**: Distributed locking configuration
  - **`type`**: Lock provider type (`azure_blob`)
  - **`connection_string`**: Azure Blob Storage connection string
  - **`container_name`**: Container name for lock files
- **`tables`**: Array of table names to monitor (schema.table format)

## Output Format

### JSON Envelope Structure
```json
{
  "table_name": "dbo.customers",
  "server_name": "localhost", 
  "changes": [
    {
      "table_name": "dbo.customers",
      "change_type": "insert",
      "data": {
        "id": 123,
        "name": "John Doe",
        "email": "john@example.com"
      },
      "timestamp": "2025-09-28T20:00:00Z",
      "lsn": "00000020:00000100:0001"
    }
  ],
  "metadata": {
    "batch_size": 1,
    "poll_interval": "5s"
  }
}
```

## Internal Architecture

### Core Components

#### 📁 `/internal/config`
**Configuration Management**
- `config.go`: JSON configuration parsing and validation
- Provides shared configuration structure with table list
- Supports time.Duration parsing for intervals

#### 📁 `/internal/cdc` 
**Change Data Capture Core**

**Checkpoint Manager** (`checkpoint_manager.go`)
- Manages LSN (Log Sequence Number) persistence per table
- Uses SQL Server table (`cdc_offsets`) to store progress
- Handles both LSN and sequence number tracking
- Provides automatic initialization and recovery

**Backoff Manager** (`backoff.go`)
- Implements exponential backoff for polling optimization
- Reduces database load when no changes are detected
- Configurable initial and maximum intervals
- Automatic reset on successful change detection

**Batch Sizer** (`batchsizer.go`)
- Calculates optimal batch sizes for CDC queries
- Considers message size limits (Service Bus/Event Hub)
- Monitors performance and adjusts dynamically
- Background sampling and optimization

#### 📁 `/internal/locking`
**Distributed Locking System**

**Locker Factory** (`locker_factory.go`)
- Creates appropriate locker instances based on configuration
- Supports multiple locking backends (currently Azure Blob)
- Handles server name extraction for hierarchical locks

**Blob Locker** (`blob_locker.go`)
- Azure Blob Storage-based distributed locking
- Uses blob leases for atomic lock acquisition
- Automatic stale lock detection and breaking (2-minute TTL)
- Lock renewal and release mechanisms

**Distributed Locker Interface** (`distributed_locker.go`)
- Common interface for all locking implementations
- Defines lock lifecycle: acquire, renew, release
- Supports lock status checking across tables

#### 📁 `/internal/db`
**Database Operations**

**Connection Manager** (`db.go`)
- SQL Server connection establishment and testing
- Connection pooling and health monitoring

**Table Metadata** (`table_metadata.go`)
- Table schema introspection
- Column metadata retrieval
- CDC table discovery and validation

#### 📁 `/internal/logging`
**Logging Infrastructure**
- `logger.go`: Simple logging wrapper for compatibility
- Provides consistent interface matching original implementation
- Structured logging with prefixes and levels

### 📁 `/pkg/types`
**Public Types and Interfaces**

**CDC Types** (`cdc.go`)
- `ChangeEvent`: Individual change record structure
- `OutputEnvelope`: JSON output wrapper with metadata
- `ChangeType`: Enumeration for insert/update/delete operations
- `TableMonitor`: Interface for table monitoring implementations

## Usage

### Running the Provider
```bash
# From configuration file
cat config.json | ./dstream-ingester-mssql

# With inline configuration
echo '{"db_connection_string":"server=localhost;database=test;trusted_connection=true","poll_interval":"5s","max_poll_interval":"5m","lock_config":{"type":"azure_blob","connection_string":"...","container_name":"locks"},"tables":["dbo.users"]}' | ./dstream-ingester-mssql
```

### Integration with DStream CLI
The provider is designed to be launched by the DStream CLI orchestrator, which handles:
- Configuration injection via stdin
- Process lifecycle management  
- Output collection and routing
- Error monitoring and restart

## Development

### Building
```bash
go build -o dstream-ingester-mssql
```

### Testing Configuration
```bash
# Test configuration parsing
cat test-config.json | ./dstream-ingester-mssql
```

### Project Structure
```
dstream-ingester-mssql/
├── main.go                              # Entry point with stdin/stdout interface
├── go.mod                               # Go module definition  
├── internal/                            # Internal implementation packages
│   ├── cdc/                            # CDC processing components
│   │   ├── checkpoint_manager.go       # LSN checkpoint persistence
│   │   ├── backoff.go                  # Exponential backoff logic
│   │   └── batchsizer.go              # Batch size optimization
│   ├── config/                         # Configuration handling
│   │   └── config.go                  # JSON configuration types
│   ├── db/                            # Database operations
│   │   ├── db.go                      # Connection management
│   │   └── table_metadata.go          # Table introspection
│   ├── locking/                       # Distributed locking
│   │   ├── distributed_locker.go      # Locking interface
│   │   ├── locker_factory.go          # Factory pattern
│   │   └── blob_locker.go             # Azure Blob implementation
│   └── logging/                       # Logging infrastructure
│       └── logger.go                  # Compatibility wrapper
├── pkg/types/                         # Public types and interfaces
│   └── cdc.go                         # CDC event structures
└── test-config.json                   # Example configuration
```

## Implementation Status

### ✅ Completed Framework
- JSON configuration parsing and validation
- Concurrent multi-table monitoring architecture
- Distributed locking with Azure Blob Storage
- Checkpoint management infrastructure
- Exponential backoff and batch sizing
- JSON envelope output formatting
- Error handling and graceful shutdown
- Complete build and test pipeline

### 🔄 Next Steps for Full CDC Implementation
- SQL Server CDC table discovery and validation
- CDC query implementation using `sys.fn_cdc_get_all_changes_*`
- Change event parsing and transformation
- LSN progression and checkpoint updates
- Integration testing with actual SQL Server CDC

## Dependencies

### Required
- **Go 1.21+**: Modern Go runtime
- **SQL Server**: Database with CDC enabled
- **Azure Blob Storage**: For distributed locking

### Go Modules
- `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`: Azure Blob operations
- `github.com/denisenkom/go-mssqldb`: SQL Server driver
- `github.com/katasec/dstream`: Logging compatibility (minimal dependency)

## Prerequisites

### SQL Server Setup
1. **Enable CDC on Database**:
   ```sql
   USE [YourDatabase]
   EXEC sys.sp_cdc_enable_db
   ```

2. **Enable CDC on Tables**:
   ```sql
   EXEC sys.sp_cdc_enable_table
     @source_schema = N'dbo',
     @source_name = N'customers',
     @role_name = NULL
   ```

3. **Verify CDC Status**:
   ```sql
   SELECT name, is_cdc_enabled FROM sys.databases WHERE name = DB_NAME()
   SELECT SCHEMA_NAME(schema_id) AS schema_name, name AS table_name, is_tracked_by_cdc 
   FROM sys.tables WHERE is_tracked_by_cdc = 1
   ```

### Azure Blob Storage
- Create storage account with blob service
- Create container for distributed locks
- Generate connection string with appropriate permissions

## Error Handling

The provider implements comprehensive error handling:

- **Database Connection Errors**: Automatic retry with exponential backoff
- **Lock Acquisition Failures**: Graceful skip with retry on next cycle  
- **CDC Query Errors**: Error logging with checkpoint preservation
- **Configuration Errors**: Immediate termination with clear error messages
- **Signal Handling**: Graceful shutdown on SIGINT/SIGTERM

## Monitoring and Observability

### Logging Output
All logs are prefixed with `[MSSQL-CDC]` and include:
- Configuration loading status
- Table monitoring lifecycle events
- Lock acquisition and release status  
- Change processing statistics
- Error conditions and recovery attempts

### Metrics (Output Envelope Metadata)
- `batch_size`: Number of changes in current batch
- `poll_interval`: Current polling interval (reflects backoff state)
- `table_name`: Source table identification
- `server_name`: Source server identification

## Contributing

This provider follows the modern DStream architecture patterns. When contributing:

1. **Maintain stdin/stdout interface**: All configuration via stdin, all output via stdout
2. **Preserve concurrent architecture**: Each table should run independently  
3. **Use distributed locking**: Ensure multi-instance safety
4. **Follow checkpoint patterns**: Maintain LSN progression tracking
5. **Add comprehensive logging**: Include monitoring and debugging information

## License

Part of the DStream ecosystem. See main DStream repository for license information.