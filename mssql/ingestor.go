package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/katasec/dstream/pkg/config"
	"github.com/katasec/dstream/pkg/db"
	"github.com/katasec/dstream/pkg/locking"
	"github.com/katasec/dstream/pkg/logging"
	"github.com/katasec/dstream/pkg/orchestrator"
	"github.com/katasec/dstream/pkg/plugins"
)

type Ingester struct {
	config        *config.Config
	dbConn        *sql.DB
	lockerFactory *locking.LockerFactory
	wg            *sync.WaitGroup
}

func New() plugins.Ingester {
	return &Ingester{
		wg: &sync.WaitGroup{},
	}
}

func (s *Ingester) Start(ctx context.Context, emit func(plugins.Event) error) error {
	logger := logging.GetLogger()
	logger.Info("Starting MSSQL ingester...")

	cfg, err := config.LoadConfig("dstream.hcl")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	s.config = cfg

	dbConn, err := db.Connect(cfg.Ingester.DBConnectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	s.dbConn = dbConn
	defer dbConn.Close()

	s.lockerFactory = locking.NewLockerFactory(
		cfg.Ingester.Locks.Type,
		cfg.Ingester.Locks.ConnectionString,
		cfg.Ingester.Locks.ContainerName,
		cfg.Ingester.DBConnectionString,
	)

	// Filter tables that are not CDC-enabled and not locked
	tablesToMonitor := s.getTablesToMonitor()
	if len(tablesToMonitor) == 0 {
		logger.Info("No available tables to monitor — exiting.")
		return nil
	}

	// Launch orchestrator
	orchestrator := orchestrator.NewTableMonitoringOrchestrator(
		s.dbConn,
		s.lockerFactory,
		tablesToMonitor,
	)

	go func() {
		if err := orchestrator.Start(ctx); err != nil {
			logger.Error("Orchestrator error", "error", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Ctrl-C detected, shutting down gracefully...")
	orchestrator.ReleaseAllLocks(context.Background())
	logger.Info("All monitoring goroutines exited cleanly.")
	return nil
}

func (s *Ingester) Stop() error {
	fmt.Println("Stopping MSSQL ingester...")
	return nil
}

func (s *Ingester) getTablesToMonitor() []config.ResolvedTableConfig {
	logger := logging.GetLogger()
	var toMonitor []config.ResolvedTableConfig
	tableNames := make([]string, len(s.config.Ingester.Tables))
	for i, t := range s.config.Ingester.Tables {
		tableNames[i] = t.Name
	}

	locked, err := s.lockerFactory.GetLockedTables(tableNames)
	if err != nil {
		logger.Error("Error fetching locked tables", "error", err)
		return toMonitor
	}
	lockedMap := map[string]bool{}
	for _, name := range locked {
		lockedMap[name] = true
	}

	for _, t := range s.config.Ingester.Tables {
		lockName := s.lockerFactory.GetLockName(t.Name)
		if lockedMap[lockName] {
			logger.Info("Table is locked — skipping", "table", t.Name)
			continue
		}
		if enabled, err := isCDCEnabled(s.dbConn, t.Name); err != nil || !enabled {
			logger.Warn("Skipping non-CDC table", "table", t.Name)
			continue
		}
		logger.Info("CDC enabled — monitoring", "table", t.Name)
		toMonitor = append(toMonitor, t)
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
