package mssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/katasec/dstream/pkg/config"
	"github.com/katasec/dstream/pkg/db"
	"github.com/katasec/dstream/pkg/locking"
	"github.com/katasec/dstream/pkg/logging"
	"github.com/katasec/dstream/pkg/plugins"
	"github.com/katasec/dstream/pkg/sqlservercdc"
)

var cfgFile = "dstream.hcl"

type Ingester struct {
	config        *config.Config
	dbConn        *sql.DB
	lockerFactory *locking.LockerFactory
	wg            *sync.WaitGroup
	heldLocks     map[string]locking.DistributedLocker
}

// emitPublisher adapts emit(...) to the cdc.ChangePublisher interface
type emitPublisher struct {
	emitFn func(evt plugins.Event) error
	table  string
}

func (e *emitPublisher) PublishChanges(changes []map[string]any) (<-chan bool, error) {
	done := make(chan bool, 1)

	evt := plugins.Event{
		"table": e.table,
		"data":  changes,
	}

	jsonPayload, _ := json.MarshalIndent(evt, "", "  ")
	log := logging.GetLogger()
	log.Info("Received event:\n" + string(jsonPayload))

	err := e.emitFn(evt)
	done <- err == nil
	return done, nil
}

func (e *emitPublisher) Close() error {
	return nil
}

func New() plugins.Ingester {
	return &Ingester{
		wg:        &sync.WaitGroup{},
		heldLocks: make(map[string]locking.DistributedLocker),
	}
}

func (s *Ingester) Start(ctx context.Context, emit func(plugins.Event) error) error {
	logger := logging.GetLogger()
	logger.Info("Starting MSSQL ingester...")

	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	s.config = cfg

	conn, err := db.Connect(cfg.Ingester.DBConnectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	s.dbConn = conn
	defer conn.Close()

	locks := cfg.Ingester.Locks
	s.lockerFactory = locking.NewLockerFactory(
		locks.Type,
		locks.ConnectionString,
		locks.ContainerName,
		cfg.Ingester.DBConnectionString,
	)

	ctx, cancel := context.WithCancel(ctx)

	for _, table := range cfg.Ingester.Tables {
		tableName := table.Name

		if !isCDCEnabled(s.dbConn, tableName) {
			logger.Warn("Skipping table — CDC not enabled", "table", tableName)
			continue
		}

		lockName := s.lockerFactory.GetLockName(tableName)
		locker, err := s.lockerFactory.CreateLocker(lockName)
		if err != nil {
			logger.Error("Failed to create locker", "table", tableName, "error", err)
			continue
		}

		leaseID, err := locker.AcquireLock(ctx, lockName)
		if err != nil {
			logger.Info("Table already locked", "table", tableName)
			continue
		}
		logger.Debug("Acquired lease", "leaseID", leaseID)

		s.heldLocks[lockName] = locker

		// Convert polling intervals
		pollInterval, err := time.ParseDuration(table.PollInterval)
		if err != nil {
			return fmt.Errorf("invalid pollInterval for table %s: %w", tableName, err)
		}
		maxPollInterval, err := time.ParseDuration(table.MaxPollInterval)
		if err != nil {
			return fmt.Errorf("invalid maxPollInterval for table %s: %w", tableName, err)
		}

		publisher := &emitPublisher{
			emitFn: emit,
			table:  tableName,
		}

		monitor := sqlservercdc.NewSQLServerTableMonitor(
			s.dbConn,
			tableName,
			pollInterval,
			maxPollInterval,
			publisher,
		)

		s.wg.Add(1)
		go func(tName string) {
			defer s.wg.Done()
			if err := monitor.MonitorTable(ctx); err != nil {
				logger.Error("Monitor failed", "table", tName, "error", err)
			}
		}(tableName)
	}

	// Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Ctrl-C detected, shutting down gracefully...")
	cancel()

	// Release all held locks
	for lockName, locker := range s.heldLocks {
		logger.Info("Releasing lock", "lock", lockName)
		err := locker.ReleaseLock(context.Background(), lockName, "")
		if err != nil {
			logger.Error("Failed to release lock", "lock", lockName, "error", err)
		}
	}

	s.wg.Wait()
	logger.Info("All monitoring goroutines exited cleanly.")
	return nil
}

func (s *Ingester) Stop() error {
	fmt.Println("Stopping MSSQL ingester...")
	return nil
}

func isCDCEnabled(conn *sql.DB, tableName string) bool {
	query := `
		SELECT COUNT(*) 
		FROM cdc.change_tables 
		WHERE source_object_id = OBJECT_ID(@p1);
	`
	var count int
	_ = conn.QueryRow(query, tableName).Scan(&count)
	return count > 0
}
