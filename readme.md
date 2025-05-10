# 📦 dstream-ingester-mssql

This plugin provides **Change Data Capture (CDC)** ingestion from **SQL Server** for the `dstream` real-time data pipeline framework.

It monitors SQL Server CDC tables and publishes change events (inserts, updates, deletes) to the `dstream` runtime via a standard publisher interface.

---

## ✨ Features

- 💡 Monitors only CDC-enabled tables
- 🧠 Distributed locking to prevent duplicate ingestion
- ⏱️ Resumable via LSN checkpoints
- 📤 Publishes events to downstream sinks (e.g., Azure Service Bus, Parquet)
- ⚙️ Built with the `TableMonitoringOrchestrator` pattern for scalability
- 🚀 Easy to swap out for other sources like Postgres, MySQL, Kafka

---

## 🛠️ Project Structure

| Folder / File | Purpose |
|---------------|---------|
| `main.go` | Entry point for plugin |
| `ingester.go` | Plugin wiring: lock factory, table monitor factory |
| `monitor/sqlserver_table_monitor.go` | SQL Server-specific CDC poller |
| `pkg/orchestrator/` | Shared orchestration logic for table polling (soon reusable!) |
| `pkg/config/` | Table-level and global configuration model |

---

## 🧪 Running Locally

```bash
go run main.go --config dstream.yaml
