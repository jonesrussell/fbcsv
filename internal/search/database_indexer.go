package search

import (
	"context"
	"fmt"
	"sync"

	"csv-search/internal/database"
	"csv-search/internal/models"
)

// DatabaseIndexer handles indexing operations using SQLite
type DatabaseIndexer struct {
	db       *database.Database
	importer *database.Importer
	config   *models.Config
	progress *models.IndexProgress
	mu       sync.RWMutex
	indexed  bool
}

// NewDatabaseIndexer creates a new database indexer
func NewDatabaseIndexer(db *database.Database, config *models.Config) *DatabaseIndexer {
	importer := database.NewImporter(db, config)

	return &DatabaseIndexer{
		db:       db,
		importer: importer,
		config:   config,
		progress: &models.IndexProgress{
			ProcessedRows: 0,
			TotalRows:     0,
			Progress:      0.0,
			IsComplete:    true, // Assume complete initially
		},
		indexed: true, // Assume indexed initially
	}
}

// BuildIndex builds the search index from CSV file using SQLite
func (i *DatabaseIndexer) BuildIndex(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Check if we already have indexed data
	stats, err := i.db.GetFileStats()
	if err != nil {
		return fmt.Errorf("failed to check existing index: %w", err)
	}

	// If we have stats and the file path matches, we might already be indexed
	if stats != nil {
		// For now, always reindex to ensure data is fresh
		// In a production system, you might want to check file modification time
	}

	// Set up progress tracking
	i.progress = &models.IndexProgress{
		ProcessedRows: 0,
		TotalRows:     0,
		Progress:      0.0,
		IsComplete:    false,
	}

	// Import CSV data
	err = i.importer.ImportCSV(ctx, func(progress *models.IndexProgress) {
		i.mu.Lock()
		i.progress = progress
		i.mu.Unlock()
	})

	if err != nil {
		i.progress.IsComplete = true
		i.progress.Error = err.Error()
		return fmt.Errorf("failed to import CSV: %w", err)
	}

	i.indexed = true
	return nil
}

// GetProgress returns the current indexing progress
func (i *DatabaseIndexer) GetProgress() *models.IndexProgress {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Return a copy to avoid race conditions
	progress := *i.progress
	return &progress
}

// GetFileStats returns file statistics
func (i *DatabaseIndexer) GetFileStats() *models.FileStats {
	stats, err := i.db.GetFileStats()
	if err != nil {
		return nil
	}
	return stats
}

// GetColumns returns available columns
func (i *DatabaseIndexer) GetColumns() []string {
	stats := i.GetFileStats()
	if stats == nil {
		return []string{}
	}

	columns := make([]string, len(stats.Columns))
	for i, col := range stats.Columns {
		columns[i] = col.Name
	}

	return columns
}

// IsIndexed returns whether the index is ready
func (i *DatabaseIndexer) IsIndexed() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if !i.indexed {
		return false
	}

	// Also check if we have data in the database
	stats, err := i.db.GetFileStats()
	if err != nil || stats == nil {
		return false
	}

	return stats.RowCount > 0
}

// Close closes the database connection
func (i *DatabaseIndexer) Close() error {
	return i.db.Close()
}
