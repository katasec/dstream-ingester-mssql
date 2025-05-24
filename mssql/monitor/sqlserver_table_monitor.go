package monitor

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/katasec/dstream/pkg/cdc"
)

// SqlServerTableMonitor manages SQL Server CDC monitoring for a specific table
type SqlServerTableMonitor struct {
	dbConn          *sql.DB
	tableName       string
	pollInterval    time.Duration
	maxPollInterval time.Duration
	lastLSNs        map[string][]byte
	lsnMutex        sync.Mutex
	checkpointMgr   *CheckpointManager
	publisher       cdc.ChangePublisher
	columnNames     []string    // Cached column names
	batchSizer      *BatchSizer // Determines optimal batch size
}

// NewSQLServerTableMonitor initializes a new SqlServerTableMonitor
func NewSQLServerTableMonitor(dbConn *sql.DB, tableName string, pollInterval, maxPollInterval time.Duration, publisher cdc.ChangePublisher) *SqlServerTableMonitor {
	checkpointMgr := NewCheckpointManager(dbConn, tableName)

	// Fetch column names once and store them in the struct
	columns, err := fetchColumnNames(dbConn, tableName)
	if err != nil {
		log.Info("Failed to fetch column names", "table", tableName, "error", err)
		os.Exit(1)
	}

	// Create a BatchSizer with Standard SKU limit by default
	batchSizer := NewBatchSizer(dbConn, tableName, StandardSKULimit)

	return &SqlServerTableMonitor{
		dbConn:          dbConn,
		tableName:       tableName,
		pollInterval:    pollInterval,
		maxPollInterval: maxPollInterval,
		lastLSNs:        make(map[string][]byte),
		checkpointMgr:   checkpointMgr,
		columnNames:     columns,
		publisher:       publisher,
		batchSizer:      batchSizer,
	}
}

// MonitorTable continuously monitors the specified table
func (m *SqlServerTableMonitor) MonitorTable(ctx context.Context) error {
	err := m.checkpointMgr.InitializeCheckpointTable()
	if err != nil {
		return fmt.Errorf("error initializing checkpoint table: %w", err)
	}

	// Load last LSN for this table
	initialLSN, err := m.checkpointMgr.LoadLastLSN()
	if err != nil {
		return fmt.Errorf("error loading last LSN for table %s: %w", m.tableName, err)
	}

	// Multiple goroutines may access lastLSNs map, so lock it
	m.lsnMutex.Lock()
	m.lastLSNs[m.tableName] = initialLSN
	m.lsnMutex.Unlock()

	// Start the batch sizer to calculate optimal batch sizes
	err = m.batchSizer.Start(ctx)
	if err != nil {
		return fmt.Errorf("error starting batch sizer: %w", err)
	}

	// Initialize the backoff manager
	backoff := NewBackoffManager(m.pollInterval, m.maxPollInterval)

	// Begin monitoring loop
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			log.Info("Stopping monitoring due to context cancellation", "table", m.tableName)
			return ctx.Err()
		default:
		}
		log.Info("Polling changes for table", "table", m.tableName, "lsn", hex.EncodeToString(m.lastLSNs[m.tableName]))
		changes, newLSN, err := m.fetchCDCChanges(m.lastLSNs[m.tableName])

		if err != nil {
			log.Info("Error fetching changes", "table", m.tableName, "error", err)
			time.Sleep(backoff.GetInterval()) // Wait with current interval on error
			continue
		}

		if len(changes) > 0 {
			log.Info("Changes detected, publishing...", "table", m.tableName, "changeCount", len(changes))

			// Publish all changes as a single batch
			doneChan, err := m.publisher.PublishChanges(changes)
			if err != nil {
				log.Error("Failed to publish batch", "error", err, "changeCount", len(changes))
				// Continue to next poll cycle without updating LSN
				time.Sleep(backoff.GetInterval())
				continue
			}

			// Wait for publish confirmation
			if success := <-doneChan; !success {
				log.Error("Failed to publish batch")
				// Continue to next poll cycle without updating LSN
				time.Sleep(backoff.GetInterval())
				continue
			}

			// Update last LSN and reset polling interval
			m.lsnMutex.Lock()
			m.lastLSNs[m.tableName] = newLSN
			m.lsnMutex.Unlock()

			// Update the checkpoint in the database
			if err := m.checkpointMgr.SaveLastLSN(newLSN); err != nil {
				log.Error("Failed to save checkpoint", "error", err)
			}

			backoff.ResetInterval() // Reset interval after detecting changes

		} else {
			// If no changes, increase the polling interval (backoff)
			backoff.IncreaseInterval()
			log.Info("No changes found", "table", m.tableName, "nextPollIn", backoff.GetInterval())
		}

		time.Sleep(backoff.GetInterval())
	}
}

// fetchCDCChanges queries CDC changes and returns relevant events as a slice of maps
func (monitor *SqlServerTableMonitor) fetchCDCChanges(lastLSN []byte) ([]map[string]interface{}, []byte, error) {
	log.Info("Polling changes", "table", monitor.tableName, "lsn", hex.EncodeToString(lastLSN))

	// Get the optimal batch size from BatchSizer
	batchSize := monitor.batchSizer.GetBatchSize()
	//log.Info("Using batch size", "table", monitor.tableName, "batchSize", batchSize)

	// Use cached column names
	columnList := "ct.__$start_lsn, ct.__$operation"
	if len(monitor.columnNames) > 0 {
		columnList += ", " + strings.Join(monitor.columnNames, ", ")
	}

	query := fmt.Sprintf(`
        SELECT TOP(%d) %s
        FROM cdc.dbo_%s_CT AS ct
        WHERE ct.__$start_lsn > @lastLSN
        ORDER BY ct.__$start_lsn
    `, batchSize, columnList, monitor.tableName)

	log.Debug(query)
	rows, err := monitor.dbConn.Query(query, sql.Named("lastLSN", lastLSN))
	log.Debug("Query executed", "query", query, "lastLSN", hex.EncodeToString(lastLSN))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query CDC table for %s: %w", monitor.tableName, err)
	}
	defer rows.Close()

	changes := []map[string]interface{}{}
	var latestLSN []byte

	for rows.Next() {

		// Prepare a slice of pointers for the columns
		columnData, lsn, operation := monitor.prepareScanTargets()

		// Scan the row into the columnData slice
		if err := rows.Scan(columnData...); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Determine the operation type as a string
		operationType, ok := getOperationType(operation)
		if !ok {
			continue // Unknown operation, skip it
		}

		// Organize metadata and data separately in the output
		data := extractColumnData(monitor.columnNames, columnData)

		change := map[string]interface{}{
			"metadata": map[string]interface{}{
				"TableName":     monitor.tableName,
				"LSN":           hex.EncodeToString(*lsn),
				"OperationID":   operation,
				"OperationType": operationType,
			},
			"data": data,
		}

		changes = append(changes, change)
		latestLSN = *lsn
	}

	// Return the changes and latest LSN without saving the checkpoint yet
	// The checkpoint will be saved in MonitorTable after successful publishing

	return changes, latestLSN, nil
}

// fetchColumnNames fetches column names for a specified table
func fetchColumnNames(db *sql.DB, tableName string) ([]string, error) {
	// Get the capture instance name for the table
	query := `
		SELECT COLUMN_NAME 
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_NAME = @tableName 
		AND TABLE_SCHEMA = 'dbo'`

	log.Debug(strings.Replace(query, "@tableName", "'"+tableName+"'", 1))

	rows, err := db.Query(query, sql.Named("tableName", tableName))

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		log.Debug("Found column", "name", columnName)
		columns = append(columns, columnName)
	}
	log.Info("Total columns found", "table", tableName, "columns", columns)
	return columns, rows.Err()
}

func (monitor *SqlServerTableMonitor) prepareScanTargets() ([]any, *[]byte, *int) {

	var lsn []byte
	var operation int

	// Prepare a slice of pointers for the columns
	columnData := make([]any, len(monitor.columnNames)+2)
	columnData[0] = &lsn
	columnData[1] = &operation

	// Prepare a slice of sql.NullString for the rest of the columns
	columnValues := make([]sql.NullString, len(monitor.columnNames))
	for i := range monitor.columnNames {
		columnData[i+2] = &columnValues[i]
	}

	return columnData, &lsn, &operation
}

func getOperationType(op *int) (string, bool) {
	if op == nil {
		return "", false
	}
	switch *op {
	case 2:
		return "Insert", true
	case 4:
		return "Update", true
	case 1:
		return "Delete", true
	default:
		return "", false
	}
}

func extractColumnData(columnNames []string, columnData []any) map[string]any {

	// Create a map to hold the column data
	data := make(map[string]any, len(columnNames))

	// Note that the first two columns are LSN and operation
	// The rest are the actual column values
	// We start from index 2 to skip LSN and operation
	for i, name := range columnNames {
		raw := columnData[i+2]
		columnValue, ok := raw.(*sql.NullString)

		if ok && columnValue.Valid {
			data[name] = columnValue.String
		} else {
			data[name] = nil
		}
	}

	return data
}
