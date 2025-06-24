package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/katasec/dstream-ingester-mssql/mssql/monitor"
	"github.com/katasec/dstream/pkg/config"
	"github.com/katasec/dstream/pkg/db"
	"github.com/katasec/dstream/pkg/locking"
	"github.com/katasec/dstream/pkg/orchestrator"
	"github.com/katasec/dstream/pkg/plugins"
)

// Ingester is an internal helper used by StartFromConfig.
// It no longer implements the (removed) plugins.Ingester interface.
type Ingester struct {
	config        *IngesterConfig
	dbConn        *sql.DB
	lockerFactory *locking.LockerFactory
	wg            *sync.WaitGroup
}

// Start kicks off monitoring and publishing for the configured tables.
func (s *Ingester) Start(ctx context.Context, emit func(plugins.Event) error) error {
	logger := GetLogger()
	logger.Info("Starting MSSQL ingester...")

	// Initialize the monitor package's logger
	monitor.SetLogger(logger)

	// Ensure config is injected
	if s.config == nil {
		return fmt.Errorf("ingester config not set")
	}
	cfg := s.config

	// ---------------------------------------------------------------------
	// DB connection
	// ---------------------------------------------------------------------
	dbConn, err := db.Connect(cfg.DBConnectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	s.dbConn = dbConn
	defer dbConn.Close()

	// ---------------------------------------------------------------------
	// Distributed lock factory
	// ---------------------------------------------------------------------
	s.lockerFactory = locking.NewLockerFactory(
		cfg.Lock.Type,
		cfg.Lock.ConnectionString,
		cfg.Lock.ContainerName,
		cfg.DBConnectionString,
	)

	// ---------------------------------------------------------------------
	// Determine which tables to monitor
	// ---------------------------------------------------------------------
	tablesToMonitor := s.GetTablesToMonitor()
	if len(tablesToMonitor) == 0 {
		logger.Info("No available tables to monitor — exiting.")
		return nil
	}

	// ---------------------------------------------------------------------
	// Launch orchestrator
	// ---------------------------------------------------------------------
	// Convert string table names to a slice of config.ResolvedTableConfig for the orchestrator
	var tableConfigs []config.ResolvedTableConfig
	for _, tableName := range tablesToMonitor {
		// Get polling intervals from config or use defaults
		pollInterval := "10s"     // Default
		maxPollInterval := "300s" // Default

		if s.config.Polling.Interval != "" {
			pollInterval = s.config.Polling.Interval
		}
		if s.config.Polling.MaxInterval != "" {
			maxPollInterval = s.config.Polling.MaxInterval
		}

		tableConfigs = append(tableConfigs, config.ResolvedTableConfig{
			Name:            tableName,
			PollInterval:    pollInterval,
			MaxPollInterval: maxPollInterval,
		})
	}

	genericOrch := orchestrator.NewGenericTableMonitoringOrchestrator(
		s.dbConn,
		s.lockerFactory,
		tableConfigs,
		func(table config.ResolvedTableConfig) (orchestrator.TableMonitor, error) {
			pollDur, err := time.ParseDuration(table.PollInterval)
			if err != nil {
				return nil, fmt.Errorf("invalid PollInterval for table %s: %w", table.Name, err)
			}
			maxDur, err := time.ParseDuration(table.MaxPollInterval)
			if err != nil {
				return nil, fmt.Errorf("invalid MaxPollInterval for table %s: %w", table.Name, err)
			}
			return monitor.NewSQLServerTableMonitor(
				s.dbConn,
				table.Name,
				pollDur,
				maxDur,
				&pluginPublisher{emit: emit},
			), nil
		},
	)

	// Start the orchestrator directly in the main thread
	// The context will be cancelled by RunWithGracefulShutdown when a signal is received
	logger.Info("Starting orchestrator for tables", "tables", tablesToMonitor)

	// Create a derived context with a timeout to ensure cleanup happens
	// This ensures that even if the main context is cancelled, we have time to clean up
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a channel to detect when the main context is cancelled
	shutdownCh := make(chan struct{})
	go func() {
		<-ctx.Done() // Wait for main context to be cancelled
		logger.Info("Context cancelled, orchestrator will clean up...")
		close(shutdownCh)
	}()

	// Run the orchestrator with the main context
	err = genericOrch.Start(ctx)

	// If we get here, the orchestrator has exited

	// Check if it was due to context cancellation
	if err == context.Canceled {
		// Wait a bit to ensure locks are released and logs are flushed
		logger.Info("Orchestrator received cancellation, ensuring cleanup...")

		// Wait for either the cleanup timeout or for the shutdown to complete
		select {
		case <-cleanupCtx.Done():
			logger.Info("Cleanup timeout reached")
		case <-time.After(1 * time.Second):
			logger.Info("Cleanup delay completed")
		}

		logger.Info("Orchestrator exited cleanly")
		return nil
	} else if err != nil {
		// Real error occurred
		logger.Error("Orchestrator error", "error", err)
		return err
	}

	// If we get here, it means the orchestrator exited normally without an error
	logger.Info("Orchestrator exited cleanly")
	return nil
}

// Stop is a placeholder to satisfy future orchestrator lifecycle hooks.
func (s *Ingester) Stop() error {
	fmt.Println("Stopping MSSQL ingester...")
	return nil
}

// -----------------------------------------------------------------------------
//  Helpers
// -----------------------------------------------------------------------------

func (s *Ingester) GetTablesToMonitor() []string {
	logger := GetLogger()
	var toMonitor []string

	// Use the tables directly from our new IngesterConfig
	tableNames := s.config.Tables

	locked, err := s.lockerFactory.GetLockedTables(tableNames)
	if err != nil {
		logger.Error("Error fetching locked tables", "error", err)
		return toMonitor
	}
	lockedMap := map[string]bool{}
	for _, name := range locked {
		lockedMap[name] = true
	}

	for _, tableName := range tableNames {
		lockName := s.lockerFactory.GetLockName(tableName)
		if lockedMap[lockName] {
			logger.Info("Table is locked — skipping", "table", tableName)
			continue
		}
		if enabled, err := isCDCEnabled(s.dbConn, tableName); err != nil || !enabled {
			logger.Warn("Skipping non-CDC table", "table", tableName)
			continue
		}
		toMonitor = append(toMonitor, tableName)
	}

	return toMonitor
}

func isCDCEnabled(conn *sql.DB, tableName string) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM cdc.change_tables 
		WHERE source_object_id = OBJECT_ID(@p1);
	`
	var count int
	err := conn.QueryRow(query, tableName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check CDC for table %s: %w", tableName, err)
	}
	return count > 0, nil
}

// -----------------------------------------------------------------------------
//  Event publisher used by table monitors
// -----------------------------------------------------------------------------

type pluginPublisher struct{ emit func(plugins.Event) error }

func (p *pluginPublisher) PublishChanges(changes []map[string]interface{}) (<-chan bool, error) {
	done := make(chan bool, 1)
	go func() {
		for _, change := range changes {
			if err := p.emit(plugins.Event(change)); err != nil {
				done <- false
				return
			}
		}
		done <- true
	}()
	return done, nil
}
func (p *pluginPublisher) Close() error { return nil }
