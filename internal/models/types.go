package models

import (
	"time"
)

// SearchRequest represents the search parameters
type SearchRequest struct {
	Query   string   `json:"query" form:"q"`
	Page    int      `json:"page" form:"page"`
	Limit   int      `json:"limit" form:"limit"`
	Columns []string `json:"columns" form:"columns"`
}

// SearchResponse represents the search results
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
	Query      string         `json:"query"`
	Duration   time.Duration  `json:"duration_ms"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Row        int                 `json:"row"`
	Data       map[string]string   `json:"data"`
	Score      float64             `json:"score,omitempty"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}

// ColumnInfo represents information about a CSV column
type ColumnInfo struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
	Type  string `json:"type,omitempty"`
}

// FileStats represents statistics about the CSV file
type FileStats struct {
	RowCount    int          `json:"row_count"`
	ColumnCount int          `json:"column_count"`
	FileSize    int64        `json:"file_size_bytes"`
	IndexedAt   time.Time    `json:"indexed_at"`
	Columns     []ColumnInfo `json:"columns"`
}

// IndexProgress represents indexing progress
type IndexProgress struct {
	ProcessedRows int     `json:"processed_rows"`
	TotalRows     int     `json:"total_rows"`
	Progress      float64 `json:"progress_percent"`
	IsComplete    bool    `json:"is_complete"`
	Error         string  `json:"error,omitempty"`
}

// APIError represents an API error response
type APIError struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// Config represents application configuration
type Config struct {
	Port        string `env:"PORT" default:"8080"`
	CSVFilePath string `env:"CSV_FILE_PATH" default:"data.csv"`
	IndexPath   string `env:"INDEX_PATH" default:"index"`
	MaxResults  int    `env:"MAX_RESULTS" default:"1000"`
	PageSize    int    `env:"PAGE_SIZE" default:"50"`
	CacheSize   int    `env:"CACHE_SIZE" default:"1000"`
	EnableCORS  bool   `env:"ENABLE_CORS" default:"true"`
	LogLevel    string `env:"LOG_LEVEL" default:"info"`
}
