package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/katasec/dstream-ingester-mssql/internal/cdc"
	"github.com/katasec/dstream-ingester-mssql/internal/config"
	"github.com/katasec/dstream-ingester-mssql/internal/db"
	"github.com/katasec/dstream-ingester-mssql/internal/locking"
	"github.com/katasec/dstream-ingester-mssql/pkg/types"
)

func main() {
	log.SetPrefix("[MSSQL-CDC] ")
	log.SetFlags(log.LstdFlags)

	// Read configuration from stdin
	providerConfig, err := readConfigFromStdin()
	if err != nil {
		log.Fatalf("Failed to read configuration: %v", err)
	}

	log.Printf("Loaded configuration for %d tables", len(providerConfig.Tables))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Start monitoring all tables concurrently
	var wg sync.WaitGroup
	for _, tableName := range providerConfig.Tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()
			if err := monitorTable(ctx, providerConfig, table); err != nil {
				log.Printf("Error monitoring table %s: %v", table, err)
			}
		}(tableName)
	}

	// Wait for all table monitors to finish
	wg.Wait()
	log.Println("All table monitors stopped")
}

// readConfigFromStdin reads JSON configuration from stdin
func readConfigFromStdin() (*config.ProviderConfig, error) {
	reader := bufio.NewReader(os.Stdin)
	input, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from stdin: %v", err)
	}

	config, err := config.LoadConfigFromJSON(input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %v", err)
	}

	return config, nil
}

// monitorTable monitors a single table for CDC changes
func monitorTable(ctx context.Context, providerConfig *config.ProviderConfig, tableName string) error {
	log.Printf("Starting monitoring for table: %s", tableName)

	// Parse polling intervals
	pollInterval, err := providerConfig.GetPollInterval()
	if err != nil {
		return fmt.Errorf("invalid poll interval for table %s: %v", tableName, err)
	}

	maxPollInterval, err := providerConfig.GetMaxPollInterval()
	if err != nil {
		return fmt.Errorf("invalid max poll interval for table %s: %v", tableName, err)
	}

	// Connect to database
	database, err := db.Connect(providerConfig.DBConnectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to database for table %s: %v", tableName, err)
	}
	defer database.Close()

	// Create distributed locker
	locker := locking.NewLockerFactory(
		providerConfig.LockConfig.Type,
		providerConfig.LockConfig.ConnectionString,
		providerConfig.LockConfig.ContainerName,
		providerConfig.DBConnectionString,
	)

	// Create checkpoint manager
	checkpointManager := cdc.NewCheckpointManager(database, tableName)

	// Batch sizer would be used in actual CDC implementation
	// batchSizer := cdc.NewBatchSizer(database, tableName, 256*1024)

	// Get server name for output envelope
	serverName := config.GetServerName(providerConfig.DBConnectionString)

	// Main monitoring loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	currentInterval := pollInterval
	backoff := cdc.NewBackoffManager(pollInterval, maxPollInterval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping monitoring for table: %s", tableName)
			return nil

		case <-ticker.C:
			// Try to acquire distributed lock
			distributedLocker, err := locker.CreateLocker(tableName)
			if err != nil {
				log.Printf("Failed to create distributed locker for table %s: %v", tableName, err)
				continue
			}

			leaseID, err := distributedLocker.AcquireLock(ctx, tableName)
			if err != nil {
				log.Printf("Failed to acquire lock for table %s: %v", tableName, err)
				continue
			}

			if leaseID == "" {
				// Table is already being processed by another instance
				log.Printf("Table %s is locked by another instance, skipping", tableName)
				continue
			}

			// Process CDC changes
			changes, err := processCDCChanges(ctx, database, tableName, checkpointManager)
			if err != nil {
				log.Printf("Error processing CDC changes for table %s: %v", tableName, err)
				distributedLocker.ReleaseLock(ctx, tableName, leaseID)
				
				// Apply exponential backoff on error
				backoff.IncreaseInterval()
				currentInterval = backoff.GetInterval()
				ticker.Reset(currentInterval)
				continue
			}

			// Release the lock
			distributedLocker.ReleaseLock(ctx, tableName, leaseID)

			// Output changes if any found
			if len(changes) > 0 {
				envelope := types.OutputEnvelope{
					TableName:  tableName,
					ServerName: serverName,
					Changes:    changes,
					Metadata: map[string]interface{}{
						"batch_size":    len(changes),
						"poll_interval": currentInterval.String(),
					},
				}

				if err := outputEnvelope(envelope); err != nil {
					log.Printf("Failed to output envelope for table %s: %v", tableName, err)
				} else {
					log.Printf("Output %d changes for table %s", len(changes), tableName)
				}

				// Reset backoff on successful processing
				backoff.ResetInterval()
				currentInterval = pollInterval
				ticker.Reset(currentInterval)
			} else {
				// No changes found, apply backoff
				backoff.IncreaseInterval()
				currentInterval = backoff.GetInterval()
				ticker.Reset(currentInterval)
			}
		}
	}
}

// processCDCChanges queries for CDC changes and returns them
func processCDCChanges(ctx context.Context, database *sql.DB, tableName string, checkpointManager *cdc.CheckpointManager) ([]types.ChangeEvent, error) {
	// Get the last processed LSN
	lastLSN, err := checkpointManager.GetLastLSN()
	if err != nil {
		return nil, fmt.Errorf("failed to get last LSN: %v", err)
	}

	// Build CDC query
	// This is a simplified version - in a real implementation, you'd need to:
	// 1. Parse the table name to extract schema and table
	// 2. Query the CDC tables (e.g., cdc.dbo_TableName_CT)
	// 3. Handle different change types (Insert, Update, Delete)
	// 4. Process column metadata properly
	
	// For now, return empty changes as this is just the framework
	log.Printf("Processing CDC changes for table %s from LSN %s (placeholder implementation)", tableName, lastLSN)
	
	// TODO: Implement actual CDC query logic here
	// This would involve:
	// - Querying sys.fn_cdc_get_all_changes_* functions
	// - Processing the results into ChangeEvent structures
	// - Updating the checkpoint with new LSN
	
	return []types.ChangeEvent{}, nil
}

// outputEnvelope outputs a JSON envelope to stdout
func outputEnvelope(envelope types.OutputEnvelope) error {
	jsonData, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope to JSON: %v", err)
	}

	// Output to stdout with a newline
	fmt.Println(string(jsonData))
	return nil
}