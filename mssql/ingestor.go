package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/katasec/dstream-ingester-mssql/mssql/monitor"
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

	// Filter tables
	tablesToMonitor := s.GetTablesToMonitor()
	if len(tablesToMonitor) == 0 {
		logger.Info("No available tables to monitor — exiting.")
		return nil
	}

	// Wire up the orchestrator
	genericOrch := orchestrator.NewGenericTableMonitoringOrchestrator(
		s.dbConn,
		s.lockerFactory,
		tablesToMonitor,
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

	go func() {
		if err := genericOrch.Start(ctx); err != nil {
			logger.Error("Orchestrator error", "error", err)
		}
	}()

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Ctrl-C detected, shutting down gracefully...")
	logger.Info("All monitoring goroutines exited cleanly.")
	return nil
}

func (s *Ingester) Stop() error {
	fmt.Println("Stopping MSSQL ingester...")
	return nil
}

// Exported so it's accessible
func (s *Ingester) GetTablesToMonitor() []config.ResolvedTableConfig {
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

// Simple wrapper to adapt `emit` function to a publisher
type pluginPublisher struct {
	emit func(plugins.Event) error
}

// Wraps single-event publishing into batch mode
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

func (p *pluginPublisher) Close() error {
	return nil
}

func StartFromConfig(ctx context.Context, dbConnectionString string, tables []string) error {
	ing := New()
	return ing.Start(ctx, func(e plugins.Event) error {
		log.Printf("[EVENT] %v", e)
		return nil
	})
}
