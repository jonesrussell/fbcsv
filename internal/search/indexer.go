package search

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"csv-search/internal/models"
)

// Index represents the search index
type Index struct {
	mu            sync.RWMutex
	rows          []map[string]string
	columns       []string
	columnIndex   map[string]int
	invertedIndex map[string][]int // word -> row indices
	fileStats     *models.FileStats
	indexedAt     time.Time
}

// Indexer handles CSV indexing operations
type Indexer struct {
	index    *Index
	config   *models.Config
	progress *models.IndexProgress
	mu       sync.RWMutex
}

// NewIndexer creates a new indexer instance
func NewIndexer(config *models.Config) *Indexer {
	return &Indexer{
		index: &Index{
			rows:          make([]map[string]string, 0),
			columns:       make([]string, 0),
			columnIndex:   make(map[string]int),
			invertedIndex: make(map[string][]int),
		},
		config: config,
		progress: &models.IndexProgress{
			ProcessedRows: 0,
			TotalRows:     0,
			Progress:      0.0,
			IsComplete:    false,
		},
	}
}

// BuildIndex builds the search index from CSV file
func (i *Indexer) BuildIndex(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Reset progress
	i.progress = &models.IndexProgress{
		ProcessedRows: 0,
		TotalRows:     0,
		Progress:      0.0,
		IsComplete:    false,
	}

	// Get file info
	fileInfo, err := os.Stat(i.config.CSVFilePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Open CSV file
	file, err := os.Open(i.config.CSVFilePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Count total rows for progress tracking
	totalRows, err := i.countRows(file)
	if err != nil {
		return fmt.Errorf("failed to count rows: %w", err)
	}

	// Reset file position
	file.Seek(0, 0)

	i.progress.TotalRows = totalRows

	// Create CSV reader
	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Initialize columns
	i.index.columns = header
	i.index.columnIndex = make(map[string]int)
	for idx, col := range header {
		i.index.columnIndex[col] = idx
	}

	// Initialize inverted index
	i.index.invertedIndex = make(map[string][]int)

	// Process rows
	rowIndex := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read record at row %d: %w", rowIndex+2, err)
		}

		// Create row data map
		rowData := make(map[string]string)
		for colIdx, value := range record {
			if colIdx < len(header) {
				rowData[header[colIdx]] = value
			}
		}

		// Add to rows
		i.index.rows = append(i.index.rows, rowData)

		// Build inverted index
		i.buildInvertedIndexForRow(rowIndex, rowData)

		rowIndex++
		i.progress.ProcessedRows = rowIndex
		i.progress.Progress = float64(rowIndex) / float64(totalRows) * 100

		// Check for cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Update file stats
	i.index.fileStats = &models.FileStats{
		RowCount:    len(i.index.rows),
		ColumnCount: len(i.index.columns),
		FileSize:    fileInfo.Size(),
		IndexedAt:   time.Now(),
		Columns:     make([]models.ColumnInfo, len(i.index.columns)),
	}

	for idx, col := range i.index.columns {
		i.index.fileStats.Columns[idx] = models.ColumnInfo{
			Name:  col,
			Index: idx,
		}
	}

	i.index.indexedAt = time.Now()
	i.progress.IsComplete = true
	i.progress.Progress = 100.0

	return nil
}

// countRows counts the total number of rows in the CSV file
func (i *Indexer) countRows(file *os.File) (int, error) {
	file.Seek(0, 0)
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count - 1, scanner.Err() // Subtract 1 for header
}

// buildInvertedIndexForRow builds inverted index for a single row
func (i *Indexer) buildInvertedIndexForRow(rowIndex int, rowData map[string]string) {
	for _, value := range rowData {
		// Tokenize the value
		tokens := i.tokenize(value)
		for _, token := range tokens {
			if token != "" {
				// Add to inverted index
				if i.index.invertedIndex[token] == nil {
					i.index.invertedIndex[token] = make([]int, 0)
				}
				i.index.invertedIndex[token] = append(i.index.invertedIndex[token], rowIndex)
			}
		}
	}
}

// tokenize splits text into searchable tokens
func (i *Indexer) tokenize(text string) []string {
	// Convert to lowercase and split on whitespace and punctuation
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, ",", " ")
	text = strings.ReplaceAll(text, ".", " ")
	text = strings.ReplaceAll(text, ";", " ")
	text = strings.ReplaceAll(text, ":", " ")
	text = strings.ReplaceAll(text, "!", " ")
	text = strings.ReplaceAll(text, "?", " ")
	text = strings.ReplaceAll(text, "(", " ")
	text = strings.ReplaceAll(text, ")", " ")
	text = strings.ReplaceAll(text, "[", " ")
	text = strings.ReplaceAll(text, "]", " ")
	text = strings.ReplaceAll(text, "{", " ")
	text = strings.ReplaceAll(text, "}", " ")
	text = strings.ReplaceAll(text, "\"", " ")
	text = strings.ReplaceAll(text, "'", " ")

	// Split and filter empty strings
	words := strings.Fields(text)
	tokens := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 0 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// GetProgress returns the current indexing progress
func (i *Indexer) GetProgress() *models.IndexProgress {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.progress
}

// GetIndex returns the current index
func (i *Indexer) GetIndex() *Index {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.index
}

// GetFileStats returns file statistics
func (i *Indexer) GetFileStats() *models.FileStats {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.index.fileStats == nil {
		return nil
	}
	return i.index.fileStats
}

// GetColumns returns available columns
func (i *Indexer) GetColumns() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.index.columns
}

// IsIndexed returns true if the index is complete
func (i *Indexer) IsIndexed() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.progress.IsComplete
}
