#!/bin/bash
# Create manifest for MSSQL CDC Provider OCI artifact

set -e

TAG="${1:-v0.1.0}"
TMP_DIR="${2:-.build}"

echo "📋 Creating provider manifest…"

cat > "$TMP_DIR/provider.json" << EOF
{
  "name": "dstream-ingester-mssql",
  "version": "$TAG",
  "description": "DStream SQL Server CDC provider for capturing table changes",
  "type": "input",
  "sdk_version": "0.1.1",
  "config_schema": {
    "db_connection_string": {
      "type": "string",
      "description": "SQL Server connection string with CDC enabled"
    },
    "poll_interval": {
      "type": "string",
      "description": "Base polling interval (e.g., '5s')",
      "default": "5s"
    },
    "max_poll_interval": {
      "type": "string",
      "description": "Maximum polling interval with backoff",
      "default": "5m"
    },
    "tables": {
      "type": "array",
      "description": "List of tables to monitor (without schema prefix)"
    },
    "lock_config": {
      "type": "object",
      "description": "Distributed locking configuration"
    }
  },
  "platforms": ["linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64"]
}
EOF

echo "✅ Manifest created: $TMP_DIR/provider.json"
