package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/katasec/dstream-ingester-mssql/internal/cdc/sqlserver"
	"github.com/katasec/dstream-ingester-mssql/internal/config"
	"github.com/katasec/dstream-ingester-mssql/internal/db"
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

	// Connect to database once and share across all table monitors
	database, err := db.Connect(providerConfig.DBConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Printf("Successfully connected to database")
	log.Printf("Connected to database, will be shared across all table monitors")

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

	// Parse intervals once
	pollInterval, err := providerConfig.GetPollInterval()
	if err != nil {
		log.Fatalf("Invalid poll interval: %v", err)
	}

	maxPollInterval, err := providerConfig.GetMaxPollInterval()
	if err != nil {
		log.Fatalf("Invalid max poll interval: %v", err)
	}

	// Create simple console publisher
	publisher := &ConsolePublisher{}

	// Start monitoring all tables concurrently using production monitors
	var wg sync.WaitGroup
	for _, tableName := range providerConfig.Tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()
			monitor := sqlserver.NewSQLServerTableMonitor(database, table, pollInterval, maxPollInterval, publisher)
			if err := monitor.MonitorTable(ctx); err != nil && err != context.Canceled {
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

// ConsolePublisher implements cdc.ChangePublisher for console output
type ConsolePublisher struct{}

func (p *ConsolePublisher) PublishChanges(changes []map[string]interface{}) (<-chan bool, error) {
	done := make(chan bool, 1)
	
	// Convert to JSON and output to stdout
	for _, change := range changes {
		jsonData, err := json.Marshal(change)
		if err != nil {
			done <- false
			return done, fmt.Errorf("failed to marshal change to JSON: %v", err)
		}
		fmt.Println(string(jsonData))
	}
	
	done <- true
	return done, nil
}

func (p *ConsolePublisher) Close() error {
	return nil
}