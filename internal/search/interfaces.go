package search

import (
	"context"

	"csv-search/internal/models"
)

// IndexerInterface defines the interface for indexing operations
type IndexerInterface interface {
	BuildIndex(ctx context.Context) error
	GetProgress() *models.IndexProgress
	GetFileStats() *models.FileStats
	GetColumns() []string
	IsIndexed() bool
	Close() error
}

// SearcherInterface defines the interface for search operations
type SearcherInterface interface {
	Search(ctx context.Context, req *models.SearchRequest) (*models.SearchResponse, error)
	GetSuggestions(ctx context.Context, query string, limit int) ([]string, error)
}
