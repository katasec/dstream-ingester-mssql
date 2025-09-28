package cdc

import (
	"context"
	"database/sql"
	"log"
	"sync/atomic"
	"time"
)

const (
	defaultSampleSize       = 100
	defaultBufferFactor     = 0.2 // 20% safety margin
	defaultResampleInterval = 1 * time.Hour

	// Service Bus SKU limits
	StandardSKULimit = 256 * 1024  // 256KB
	PremiumSKULimit  = 1024 * 1024 // 1MB
)

// BatchSizer calculates and maintains optimal batch sizes for CDC reads
type BatchSizer struct {
	batchSize        atomic.Int32
	db               *sql.DB
	tableName        string
	maxMessageSize   int
	sampleSize       int
	bufferFactor     float64
	resampleInterval time.Duration

	// For monitoring/metrics
	lastSampleTime atomic.Int64
	lastSampleSize atomic.Int32
	lastAvgRowSize atomic.Int32
}

// NewBatchSizer creates a new BatchSizer instance
func NewBatchSizer(db *sql.DB, tableName string, maxMessageSize int) *BatchSizer {
	bs := &BatchSizer{
		db:               db,
		tableName:        tableName,
		maxMessageSize:   maxMessageSize,
		sampleSize:       defaultSampleSize,
		bufferFactor:     defaultBufferFactor,
		resampleInterval: defaultResampleInterval,
	}

	// Set default batch size
	bs.batchSize.Store(100)
	
	return bs
}

// Start begins the batch size monitoring
func (bs *BatchSizer) Start(ctx context.Context) error {
	// Do initial sampling
	if err := bs.updateBatchSize(); err != nil {
		log.Printf("Initial batch size calculation failed: %v", err)
		// Continue with default
	}

	// Start background monitoring
	go bs.monitor(ctx)
	return nil
}

// GetBatchSize returns the current calculated batch size
func (bs *BatchSizer) GetBatchSize() int32 {
	size := bs.batchSize.Load()
	// Never return 0 as batch size
	if size <= 0 {
		return 100 // Default to 100 if not yet calculated
	}
	return size
}

// updateBatchSize samples records and updates the batch size
func (bs *BatchSizer) updateBatchSize() error {
	// Simplified batch size calculation
	// For now, just use a reasonable default based on table
	// This can be enhanced later with actual sampling
	
	var batchSize int32 = 100  // Default batch size
	
	// Basic heuristic based on max message size
	switch {
	case bs.maxMessageSize >= PremiumSKULimit:
		batchSize = 200
	case bs.maxMessageSize >= StandardSKULimit:
		batchSize = 100
	default:
		batchSize = 50
	}

	bs.batchSize.Store(batchSize)
	bs.lastSampleTime.Store(time.Now().Unix())

	log.Printf("Updated batch size for table %s: %d", bs.tableName, batchSize)
	return nil
}

// monitor periodically updates the batch size
func (bs *BatchSizer) monitor(ctx context.Context) {
	ticker := time.NewTicker(bs.resampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := bs.updateBatchSize(); err != nil {
				log.Printf("Failed to update batch size for table %s: %v", bs.tableName, err)
			}
		}
	}
}